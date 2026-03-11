package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/replaycli"
	"google.golang.org/protobuf/proto"
)

func main() {
	var (
		dsn              string
		endpoint         string
		accessKey        string
		secretKey        string
		provider         string
		providerDeviceID string
		fromRaw          string
		toRaw            string
		limitObjects     int
		limitMessages    int
		match            string
	)

	flag.StringVar(&dsn, "dsn", "", "manifest postgres dsn")
	flag.StringVar(&endpoint, "endpoint", "127.0.0.1:19000", "minio endpoint")
	flag.StringVar(&accessKey, "access-key", "minio", "minio access key")
	flag.StringVar(&secretKey, "secret-key", "minio123", "minio secret key")
	flag.StringVar(&provider, "provider", "ecoflow", "provider")
	flag.StringVar(&providerDeviceID, "provider-device-id", "", "provider device id / ecoflow sn")
	flag.StringVar(&fromRaw, "from", "", "from RFC3339")
	flag.StringVar(&toRaw, "to", "", "to RFC3339")
	flag.IntVar(&limitObjects, "limit-objects", 5, "max objects")
	flag.IntVar(&limitMessages, "limit-messages", 200, "max decoded messages to print")
	flag.StringVar(&match, "match", "", "optional case-insensitive substring filter on payload/type/source")
	flag.Parse()

	if strings.TrimSpace(dsn) == "" || strings.TrimSpace(providerDeviceID) == "" || strings.TrimSpace(fromRaw) == "" || strings.TrimSpace(toRaw) == "" {
		log.Fatal("dsn, provider-device-id, from, and to are required")
	}

	from, err := time.Parse(time.RFC3339, fromRaw)
	if err != nil {
		log.Fatalf("parse from: %v", err)
	}
	to, err := time.Parse(time.RFC3339, toRaw)
	if err != nil {
		log.Fatalf("parse to: %v", err)
	}

	ctx := context.Background()
	manifest, err := replaycli.NewPostgresManifestStore(dsn)
	if err != nil {
		log.Fatalf("manifest: %v", err)
	}
	defer func() { _ = manifest.Close() }()

	reader, err := replaycli.NewMinIOObjectReader(replaycli.MinIOObjectReaderConfig{
		Endpoint:        endpoint,
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		Region:          "us-east-1",
		Secure:          false,
	})
	if err != nil {
		log.Fatalf("object reader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	objects, err := manifest.ListByDevices(ctx, replaycli.DeviceQuery{
		Provider:          provider,
		FromUnixMS:        from.UnixMilli(),
		ToUnixMS:          to.UnixMilli(),
		ProviderDeviceIDs: []string{providerDeviceID},
		MaxObjectsReturned: limitObjects,
	})
	if err != nil {
		log.Fatalf("list manifest objects: %v", err)
	}
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].TSMaxUnixMS > objects[j].TSMaxUnixMS
	})
	fmt.Printf("objects=%d\n", len(objects))

	printed := 0
	match = strings.ToLower(strings.TrimSpace(match))
	for _, object := range objects {
		body, err := reader.ReadObject(ctx, object.ObjectBucket, object.ObjectKey)
		if err != nil {
			log.Fatalf("read object %s/%s: %v", object.ObjectBucket, object.ObjectKey, err)
		}
		frames, err := replaycli.DecodeEnvelopeFrames(body)
		if err != nil {
			log.Fatalf("decode object %s/%s: %v", object.ObjectBucket, object.ObjectKey, err)
		}
		fmt.Printf("object %s/%s frames=%d ts=[%s..%s]\n",
			object.ObjectBucket,
			object.ObjectKey,
			len(frames),
			time.UnixMilli(object.TSMinUnixMS).UTC().Format(time.RFC3339),
			time.UnixMilli(object.TSMaxUnixMS).UTC().Format(time.RFC3339),
		)
		for _, frame := range frames {
			var env envelopev1.TelemetryEnvelope
			if err := proto.Unmarshal(frame, &env); err != nil {
				log.Fatalf("unmarshal envelope: %v", err)
			}
			payload := string(env.GetPayload())
			haystack := strings.ToLower(env.GetTypeCode() + " " + env.GetSource() + " " + payload)
			if match != "" && !strings.Contains(haystack, match) {
				continue
			}
			fmt.Printf("%s source=%s type=%s payload=%s\n",
				time.UnixMilli(env.GetIngestedTimeUnixMs()).UTC().Format(time.RFC3339Nano),
				env.GetSource(),
				env.GetTypeCode(),
				payload,
			)
			printed++
			if printed >= limitMessages {
				return
			}
		}
	}
}
