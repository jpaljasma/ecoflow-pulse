package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strconv"
	"strings"

	"tinygo.org/x/bluetooth"
)

type ecoFlowScanRecord struct {
	Known           bool
	ProtoVersion    int
	EncryptType     int
	Serial          string
	Active          bool
	SupportVerified bool
	Verified        bool
	Support5G       bool
}

type activeProbeRequest struct {
	Step          string
	Data          []byte
	PrivateScalar []byte
}

type bleCharacteristicWriter interface {
	Write([]byte) (int, error)
}

type activeProbeTransport struct {
	Name                    bleTransport
	WriteServiceUUID        string
	WriteCharacteristicUUID string
	WriteRole               string
	Write                   bleCharacteristicWriter
	HasWrite                bool
	HasNotify               bool
}

type activeProbeTransports struct {
	rfcomm activeProbeTransport
	alt    activeProbeTransport
}

func newActiveProbeTransports() *activeProbeTransports {
	return &activeProbeTransports{
		rfcomm: activeProbeTransport{Name: bleTransportRFCOMM},
		alt:    activeProbeTransport{Name: bleTransportAlt},
	}
}

func (t *activeProbeTransports) Observe(serviceUUID string, characteristicUUID string, role string, characteristic bluetooth.DeviceCharacteristic) {
	transportName := transportNameFromRole(role)
	if transportName == "" {
		return
	}
	target := &t.rfcomm
	if transportName == bleTransportAlt {
		target = &t.alt
	}
	if strings.HasSuffix(role, "_write") {
		target.WriteServiceUUID = serviceUUID
		target.WriteCharacteristicUUID = characteristicUUID
		target.WriteRole = role
		target.Write = characteristic
		target.HasWrite = true
	}
	if strings.HasSuffix(role, "_notify") {
		target.HasNotify = true
	}
}

func (t *activeProbeTransports) Select(selection bleTransport) []activeProbeTransport {
	complete := func(transport activeProbeTransport) bool {
		return transport.HasWrite && transport.HasNotify
	}
	switch selection {
	case bleTransportRFCOMM:
		if complete(t.rfcomm) {
			return []activeProbeTransport{t.rfcomm}
		}
	case bleTransportAlt:
		if complete(t.alt) {
			return []activeProbeTransport{t.alt}
		}
	case bleTransportBoth:
		var out []activeProbeTransport
		if complete(t.rfcomm) {
			out = append(out, t.rfcomm)
		}
		if complete(t.alt) {
			out = append(out, t.alt)
		}
		return out
	default:
		if complete(t.rfcomm) {
			return []activeProbeTransport{t.rfcomm}
		}
		if complete(t.alt) {
			return []activeProbeTransport{t.alt}
		}
	}
	return nil
}

func transportNameFromRole(role string) bleTransport {
	switch role {
	case "ecoflow_rfcomm_write", "ecoflow_rfcomm_notify":
		return bleTransportRFCOMM
	case "ecoflow_alt_write", "ecoflow_alt_notify":
		return bleTransportAlt
	default:
		return ""
	}
}

type activeProbeSession struct {
	exchanges        map[bleTransport]*activeProbeExchange
	loginKey         []byte
	authUserID       string
	authDeviceSerial string
}

type activeProbeExchange struct {
	transport     activeProbeTransport
	privateScalar []byte
	stage         string
	initialKey    []byte
	sessionKey    []byte
	iv            []byte
}

func newActiveProbeSession() *activeProbeSession {
	return &activeProbeSession{
		exchanges: make(map[bleTransport]*activeProbeExchange),
		loginKey:  ecoFlowSessionLoginKey,
	}
}

func (s *activeProbeSession) trackECDHExchange(transport activeProbeTransport, privateScalar []byte) {
	if s == nil || len(privateScalar) == 0 {
		return
	}
	s.exchanges[transport.Name] = &activeProbeExchange{
		transport:     transport,
		privateScalar: append([]byte(nil), privateScalar...),
		stage:         "ecdh_public_key_request",
	}
}

func sendActiveProbeRequests(transports []activeProbeTransport, options probeOptions, record ecoFlowScanRecord) ([]probeNotification, *activeProbeSession) {
	if len(transports) == 0 {
		return []probeNotification{{
			Direction: "tx",
			Step:      string(options.ActiveProbe),
			Frame:     "none",
			Detail:    "no complete EcoFlow write/notify transport matched",
		}}, nil
	}

	request, err := buildActiveProbeRequest(options.ActiveProbe, record)
	if err != nil {
		return []probeNotification{{
			Direction: "tx",
			Step:      string(options.ActiveProbe),
			Frame:     "none",
			Detail:    err.Error(),
		}}, nil
	}

	notifications := make([]probeNotification, 0, len(transports))
	session := newActiveProbeSession()
	session.authUserID = strings.TrimSpace(options.AuthUserID)
	session.authDeviceSerial = firstNonEmpty(strings.TrimSpace(options.AuthDeviceSerial), record.Serial)
	for _, transport := range transports {
		decoded := decodeEcoFlowNotification(request.Data)
		notification := probeNotification{
			Direction:          "tx",
			ServiceUUID:        transport.WriteServiceUUID,
			CharacteristicUUID: transport.WriteCharacteristicUUID,
			Role:               transport.WriteRole,
			Step:               request.Step,
			Bytes:              len(request.Data),
			Frame:              decoded.Frame,
			Detail:             decoded.Detail,
		}
		if options.RawNotifications {
			notification.ValueHex = hex.EncodeToString(request.Data)
			notification.DecodedHex = decoded.DecodedHex
		}
		if _, err := transport.Write.Write(request.Data); err != nil {
			notification.Detail = joinDetails(notification.Detail, "write_error="+err.Error())
		} else if request.Step == "ecdh_public_key_request" {
			session.trackECDHExchange(transport, request.PrivateScalar)
		}
		notifications = append(notifications, notification)
	}
	if len(session.exchanges) == 0 {
		session = nil
	}
	return notifications, session
}

func buildActiveProbeRequest(mode activeProbeMode, record ecoFlowScanRecord) (activeProbeRequest, error) {
	switch mode {
	case activeProbeAuto:
		if record.Known && record.EncryptType == 0 {
			return buildAuthStatusProbeRequest(), nil
		}
		return buildECDHProbeRequest(rand.Reader)
	case activeProbeECDH:
		return buildECDHProbeRequest(rand.Reader)
	case activeProbeAuthStatus:
		return buildAuthStatusProbeRequest(), nil
	case activeProbeNone:
		return activeProbeRequest{}, errors.New("active probe disabled")
	default:
		return activeProbeRequest{}, fmt.Errorf("unsupported active probe %q", mode)
	}
}

func buildECDHProbeRequest(random io.Reader) (activeProbeRequest, error) {
	privateScalar, publicKey, err := generateSECP160R1Key(random)
	if err != nil {
		return activeProbeRequest{}, err
	}
	payload := append([]byte{0x01, 0x00}, publicKey...)
	return activeProbeRequest{
		Step:          "ecdh_public_key_request",
		Data:          wrapEcoFlowSimpleEncPacket(payload),
		PrivateScalar: privateScalar,
	}, nil
}

func (s *activeProbeSession) HandleNotification(raw rawBLENotification, options probeOptions) []probeNotification {
	return s.HandleNotificationEvent(raw, options).Notifications
}

func (s *activeProbeSession) HandleNotificationEvent(raw rawBLENotification, options probeOptions) probeEvent {
	if s == nil {
		return probeEvent{}
	}
	transportName := transportNameFromRole(raw.Role)
	exchange := s.exchanges[transportName]
	if exchange == nil {
		return probeEvent{}
	}
	decoded := decodeEcoFlowNotification(raw.Data)
	payload := decoded.DecodedPayload
	if len(payload) == 0 {
		return probeEvent{}
	}
	switch exchange.stage {
	case "ecdh_public_key_request":
		if payload[0] != 0x01 {
			return probeEvent{}
		}
		return probeEvent{Notifications: s.handleECDHPublicKey(raw, exchange, payload, options)}
	case "session_key_info_request":
		if payload[0] != 0x02 {
			return probeEvent{}
		}
		return probeEvent{Notifications: s.handleSessionKeyInfo(raw, exchange, payload, options)}
	case "auth_status_request", "type7_packet_stream":
		return s.handleEncryptedType7Packet(raw, exchange, options)
	default:
		return probeEvent{}
	}
}

func (s *activeProbeSession) handleECDHPublicKey(
	raw rawBLENotification,
	exchange *activeProbeExchange,
	payload []byte,
	options probeOptions,
) []probeNotification {
	initialKey, iv, err := deriveType7InitialEncryption(exchange.privateScalar, payload)
	if err != nil {
		return []probeNotification{activeProbeDiagnostic(raw, options, "ecdh_shared_key", "ecoflow_session_key", "derive_error="+err.Error(), payload)}
	}
	exchange.initialKey = initialKey
	exchange.iv = iv
	exchange.stage = "session_key_info_request"

	requestData := wrapEcoFlowSimpleEncPacket([]byte{0x02})
	decoded := decodeEcoFlowNotification(requestData)
	notification := probeNotification{
		Direction:          "tx",
		ServiceUUID:        exchange.transport.WriteServiceUUID,
		CharacteristicUUID: exchange.transport.WriteCharacteristicUUID,
		Role:               exchange.transport.WriteRole,
		Step:               "session_key_info_request",
		Bytes:              len(requestData),
		Frame:              decoded.Frame,
		Detail:             decoded.Detail,
	}
	if options.RawNotifications {
		notification.ValueHex = hex.EncodeToString(requestData)
		notification.DecodedHex = decoded.DecodedHex
	}
	if _, err := exchange.transport.Write.Write(requestData); err != nil {
		notification.Detail = joinDetails(notification.Detail, "write_error="+err.Error())
	}
	return []probeNotification{notification}
}

func (s *activeProbeSession) handleSessionKeyInfo(
	raw rawBLENotification,
	exchange *activeProbeExchange,
	payload []byte,
	options probeOptions,
) []probeNotification {
	decrypted, err := decryptEcoFlowType7(payload[1:], exchange.initialKey, exchange.iv)
	if err != nil {
		return []probeNotification{activeProbeDiagnostic(raw, options, "session_key_info_decrypted", "ecoflow_session_key", "decrypt_error="+err.Error(), payload)}
	}
	detail := "decrypted"
	if len(decrypted) >= 18 {
		detail = "srand_seed"
	}
	notification := probeNotification{
		Direction:          "rx",
		ServiceUUID:        raw.ServiceUUID,
		CharacteristicUUID: raw.CharacteristicUUID,
		Role:               raw.Role,
		Step:               "session_key_info_decrypted",
		Bytes:              len(decrypted),
		Frame:              "ecoflow_session_key",
		Detail:             detail,
	}
	if options.RawNotifications {
		notification.DecodedHex = hex.EncodeToString(decrypted)
	}
	notifications := []probeNotification{notification}

	if len(decrypted) < 18 {
		return notifications
	}
	if exchange.transport.Write == nil {
		return notifications
	}
	sessionKey, err := deriveEcoFlowSessionKeyWithLoginKey(decrypted[16:18], decrypted[:16], s.loginKey)
	if err != nil {
		notifications = append(notifications, activeProbeDiagnostic(raw, options, "auth_status_request", "ecoflow_session_key", "session_key_error="+err.Error(), decrypted))
		return notifications
	}
	exchange.sessionKey = sessionKey
	exchange.stage = "auth_status_request"
	notifications = append(notifications, s.sendEncryptedAuthStatus(exchange, options))
	return notifications
}

func (s *activeProbeSession) handleEncryptedType7Packet(
	raw rawBLENotification,
	exchange *activeProbeExchange,
	options probeOptions,
) probeEvent {
	plaintext, err := decryptEcoFlowEncPacketPayload(raw.Data, exchange.sessionKey, exchange.iv)
	if err != nil {
		return probeEvent{Notifications: []probeNotification{
			activeProbeDiagnostic(raw, options, "type7_packet_decrypted", "ecoflow_packet", "decrypt_error="+err.Error(), raw.Data),
		}}
	}
	decoded := decodeEcoFlowNotification(plaintext)
	notification := probeNotification{
		Direction:          "rx",
		ServiceUUID:        raw.ServiceUUID,
		CharacteristicUUID: raw.CharacteristicUUID,
		Role:               raw.Role,
		Step:               "type7_packet_decrypted",
		Bytes:              len(plaintext),
		Frame:              decoded.Frame,
		Detail:             decoded.Detail,
	}
	if decoded.Packet != nil {
		notification.Packet = formatEcoFlowPacketSummary(*decoded.Packet)
	}
	if options.RawNotifications {
		notification.ValueHex = hex.EncodeToString(raw.Data)
		notification.DecodedHex = hex.EncodeToString(plaintext)
	}
	notifications := []probeNotification{notification}
	if exchange.stage == "auth_status_request" && isEcoFlowAuthStatusResponse(decoded.Packet) {
		notifications = append(notifications, s.sendAuthRequest(exchange, options))
	}
	exchange.stage = "type7_packet_stream"
	return probeEvent{
		Notifications: notifications,
		Metrics:       decoded.Metrics,
	}
}

func (s *activeProbeSession) sendEncryptedAuthStatus(exchange *activeProbeExchange, options probeOptions) probeNotification {
	plaintext := buildEcoFlowV3Packet(0x21, 0x35, 0x35, 0x89, nil)
	requestData, err := wrapEcoFlowEncryptedProtocolPacket(plaintext, exchange.sessionKey, exchange.iv)
	if err != nil {
		return probeNotification{
			Direction: "tx",
			Step:      "auth_status_request",
			Frame:     "ecoflow_enc_packet",
			Detail:    "encrypt_error=" + err.Error(),
		}
	}
	notification := encryptedActiveProbeTXNotification(exchange.transport, "auth_status_request", requestData, plaintext, options)
	if _, err := exchange.transport.Write.Write(requestData); err != nil {
		notification.Detail = joinDetails(notification.Detail, "write_error="+err.Error())
	}
	return notification
}

func (s *activeProbeSession) sendAuthRequest(exchange *activeProbeExchange, options probeOptions) probeNotification {
	if strings.TrimSpace(s.authUserID) == "" || strings.TrimSpace(s.authDeviceSerial) == "" {
		return probeNotification{
			Direction:          "tx",
			ServiceUUID:        exchange.transport.WriteServiceUUID,
			CharacteristicUUID: exchange.transport.WriteCharacteristicUUID,
			Role:               exchange.transport.WriteRole,
			Step:               "auth_request",
			Frame:              "ecoflow_auth",
			Detail:             "auth_skipped=missing_user_id_or_device_serial",
		}
	}
	payloadHash := md5.Sum([]byte(s.authUserID + s.authDeviceSerial))
	payload := []byte(strings.ToUpper(hex.EncodeToString(payloadHash[:])))
	plaintext := buildEcoFlowV3Packet(0x21, 0x35, 0x35, 0x86, payload)
	requestData, err := wrapEcoFlowEncryptedProtocolPacket(plaintext, exchange.sessionKey, exchange.iv)
	if err != nil {
		return probeNotification{
			Direction: "tx",
			Step:      "auth_request",
			Frame:     "ecoflow_enc_packet",
			Detail:    "encrypt_error=" + err.Error(),
		}
	}
	notification := encryptedActiveProbeTXNotification(exchange.transport, "auth_request", requestData, plaintext, options)
	notification.Detail = joinDetails(notification.Detail, "auth_payload=redacted")
	notification.DecodedHex = ""
	if _, err := exchange.transport.Write.Write(requestData); err != nil {
		notification.Detail = joinDetails(notification.Detail, "write_error="+err.Error())
	}
	exchange.stage = "auth_request"
	return notification
}

func isEcoFlowAuthStatusResponse(packet *ecoFlowPacketSummary) bool {
	return packet != nil && packet.Command == "src=0x35,dst=0x21,cmd_set=0x35,cmd_id=0x89"
}

func encryptedActiveProbeTXNotification(
	transport activeProbeTransport,
	step string,
	data []byte,
	plaintext []byte,
	options probeOptions,
) probeNotification {
	decoded := decodeEcoFlowNotification(data)
	plaintextDecoded := decodeEcoFlowNotification(plaintext)
	detail := joinDetails(decoded.Detail, "encrypted_payload")
	if plaintextDecoded.Packet != nil {
		detail = joinDetails(detail, "plaintext_packet="+formatEcoFlowPacketSummary(*plaintextDecoded.Packet))
	}
	notification := probeNotification{
		Direction:          "tx",
		ServiceUUID:        transport.WriteServiceUUID,
		CharacteristicUUID: transport.WriteCharacteristicUUID,
		Role:               transport.WriteRole,
		Step:               step,
		Bytes:              len(data),
		Frame:              decoded.Frame,
		Detail:             detail,
	}
	if options.RawNotifications {
		notification.ValueHex = hex.EncodeToString(data)
		notification.DecodedHex = hex.EncodeToString(plaintext)
	}
	return notification
}

func activeProbeDiagnostic(
	raw rawBLENotification,
	options probeOptions,
	step string,
	frame string,
	detail string,
	decodedPayload []byte,
) probeNotification {
	notification := probeNotification{
		Direction:          "rx",
		ServiceUUID:        raw.ServiceUUID,
		CharacteristicUUID: raw.CharacteristicUUID,
		Role:               raw.Role,
		Step:               step,
		Bytes:              len(decodedPayload),
		Frame:              frame,
		Detail:             detail,
	}
	if options.RawNotifications {
		notification.ValueHex = hex.EncodeToString(raw.Data)
		notification.DecodedHex = hex.EncodeToString(decodedPayload)
	}
	return notification
}

func deriveType7InitialEncryption(privateScalar []byte, payload []byte) ([]byte, []byte, error) {
	if len(privateScalar) == 0 {
		return nil, nil, errors.New("missing private scalar")
	}
	if len(payload) < 3 || payload[0] != 0x01 {
		return nil, nil, errors.New("not an ECDH public key payload")
	}
	keySize := ecoFlowECDHTypeSize(payload[2])
	if len(payload) < 3+keySize {
		return nil, nil, fmt.Errorf("short ECDH public key payload: have %d want %d", len(payload)-3, keySize)
	}
	if keySize != 40 {
		return nil, nil, fmt.Errorf("unsupported ECDH public key size %d", keySize)
	}
	curve := newSECP160R1Curve()
	publicKey := payload[3 : 3+keySize]
	x := new(big.Int).SetBytes(publicKey[:20])
	y := new(big.Int).SetBytes(publicKey[20:])
	if !curve.isOnCurve(x, y) {
		return nil, nil, errors.New("device public key is not on secp160r1")
	}
	sharedX, _ := curve.scalarMult(x, y, privateScalar)
	sharedKey := leftPadBytes(sharedX, 20)
	iv := md5.Sum(sharedKey)
	return append([]byte(nil), sharedKey[:16]...), append([]byte(nil), iv[:]...), nil
}

func ecoFlowECDHTypeSize(ecdhType byte) int {
	switch ecdhType {
	case 1:
		return 52
	case 2:
		return 56
	case 3, 4:
		return 64
	default:
		return 40
	}
}

func decryptEcoFlowType7(ciphertext []byte, key []byte, iv []byte) ([]byte, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("invalid AES key length %d", len(key))
	}
	if len(iv) != aes.BlockSize {
		return nil, fmt.Errorf("invalid AES IV length %d", len(iv))
	}
	aligned := len(ciphertext) - len(ciphertext)%aes.BlockSize
	if aligned == 0 {
		return append([]byte(nil), ciphertext...), nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	decrypted := make([]byte, aligned)
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(decrypted, ciphertext[:aligned])
	if unpadded, ok := pkcs7Unpad(decrypted, aes.BlockSize); ok {
		return unpadded, nil
	}
	return decrypted, nil
}

func decryptEcoFlowEncPacketPayload(data []byte, key []byte, iv []byte) ([]byte, error) {
	if len(data) < 8 || data[0] != 0x5a || data[1] != 0x5a {
		return nil, errors.New("invalid EncPacket prefix")
	}
	payloadLength := int(binary.LittleEndian.Uint16(data[4:6]))
	if payloadLength < 2 {
		return nil, errors.New("invalid EncPacket payload length")
	}
	end := 6 + payloadLength
	if end > len(data) {
		return nil, errors.New("incomplete EncPacket")
	}
	if gotCRC, wantCRC := ecoFlowCRC16(data[:end-2]), binary.LittleEndian.Uint16(data[end-2:end]); gotCRC != wantCRC {
		return nil, errors.New("EncPacket crc16 mismatch")
	}
	return decryptEcoFlowType7(data[6:end-2], key, iv)
}

func encryptEcoFlowType7(plaintext []byte, key []byte, iv []byte) ([]byte, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("invalid AES key length %d", len(key))
	}
	if len(iv) != aes.BlockSize {
		return nil, fmt.Errorf("invalid AES IV length %d", len(iv))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := append([]byte(nil), plaintext...)
	padded = append(padded, bytesRepeat(byte(padding), padding)...)
	encrypted := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, padded)
	return encrypted, nil
}

func wrapEcoFlowEncryptedProtocolPacket(plaintext []byte, key []byte, iv []byte) ([]byte, error) {
	encrypted, err := encryptEcoFlowType7(plaintext, key, iv)
	if err != nil {
		return nil, err
	}
	packet := []byte{0x5a, 0x5a, 0x10, 0x01}
	packet = binary.LittleEndian.AppendUint16(packet, uint16(len(encrypted)+2))
	packet = append(packet, encrypted...)
	packet = binary.LittleEndian.AppendUint16(packet, ecoFlowCRC16(packet))
	return packet, nil
}

func deriveEcoFlowSessionKeyWithLoginKey(seed []byte, srand []byte, loginKey []byte) ([]byte, error) {
	if len(seed) != 2 {
		return nil, fmt.Errorf("invalid seed length %d", len(seed))
	}
	if len(srand) < 16 {
		return nil, fmt.Errorf("invalid srand length %d", len(srand))
	}
	pos := int(seed[0])*0x10 + int(byte(seed[1]-1))*0x100
	if pos < 0 || pos+16 > len(loginKey) {
		return nil, fmt.Errorf("seed index %d outside login key table", pos)
	}
	data := make([]byte, 0, 32)
	data = append(data, loginKey[pos:pos+16]...)
	data = append(data, srand[:16]...)
	sum := md5.Sum(data)
	return append([]byte(nil), sum[:]...), nil
}

func bytesRepeat(value byte, count int) []byte {
	out := make([]byte, count)
	for i := range out {
		out[i] = value
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, bool) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return data, false
	}
	padding := int(data[len(data)-1])
	if padding == 0 || padding > blockSize || padding > len(data) {
		return data, false
	}
	for _, value := range data[len(data)-padding:] {
		if int(value) != padding {
			return data, false
		}
	}
	return data[:len(data)-padding], true
}

func buildAuthStatusProbeRequest() activeProbeRequest {
	return activeProbeRequest{
		Step: "auth_status_request",
		Data: buildEcoFlowV3Packet(0x21, 0x35, 0x35, 0x89, nil),
	}
}

func wrapEcoFlowSimpleEncPacket(payload []byte) []byte {
	packet := []byte{0x5a, 0x5a, 0x00, 0x01}
	packet = binary.LittleEndian.AppendUint16(packet, uint16(len(payload)+2))
	packet = append(packet, payload...)
	packet = binary.LittleEndian.AppendUint16(packet, ecoFlowCRC16(packet))
	return packet
}

func buildEcoFlowV3Packet(src byte, dst byte, cmdSet byte, cmdID byte, payload []byte) []byte {
	packet := []byte{0xaa, 0x03}
	packet = binary.LittleEndian.AppendUint16(packet, uint16(len(payload)))
	packet = append(packet, ecoFlowCRC8(packet))
	packet = append(packet,
		0x0d,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00,
		src, dst,
		0x01, 0x01,
		cmdSet, cmdID,
	)
	packet = append(packet, payload...)
	packet = binary.LittleEndian.AppendUint16(packet, ecoFlowCRC16(packet))
	return packet
}

func ecoFlowScanRecordFromDevice(device discoveredBLEDevice) (ecoFlowScanRecord, bool) {
	for key, value := range device.ManufacturerData {
		if !strings.EqualFold(key, "0xb5b5") {
			continue
		}
		data, err := hex.DecodeString(value)
		if err != nil {
			return ecoFlowScanRecord{}, false
		}
		return parseEcoFlowScanRecord(data)
	}
	return ecoFlowScanRecord{}, false
}

func parseEcoFlowScanRecord(data []byte) (ecoFlowScanRecord, bool) {
	if len(data) < 17 {
		return ecoFlowScanRecord{}, false
	}
	flags := byte(0b00111000)
	if len(data) > 22 {
		flags = data[22]
	}
	status := byte(0)
	if len(data) > 17 {
		status = data[17]
	}
	record := ecoFlowScanRecord{
		Known:           true,
		ProtoVersion:    int(data[0]),
		EncryptType:     int((flags & 0b00111000) >> 3),
		Serial:          printableEcoFlowSerial(data[1:minInt(len(data), 17)]),
		Active:          ((status >> 7) & 0x01) == 1,
		SupportVerified: (flags & 0b00000010) != 0,
		Verified:        (flags & 0b00000100) != 0,
		Support5G:       ((flags >> 6) & 0b00000001) != 0,
	}
	return record, true
}

func printableEcoFlowSerial(data []byte) string {
	serial := strings.TrimRight(strings.TrimSpace(string(data)), "\x00")
	if serial == "" {
		return ""
	}
	for _, r := range serial {
		if r < 32 || r > 126 {
			return ""
		}
	}
	return serial
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func manufacturerScanRecordMetrics(device discoveredBLEDevice) []probeMetric {
	record, ok := ecoFlowScanRecordFromDevice(device)
	if !ok {
		return nil
	}
	return []probeMetric{
		{Name: "manufacturer_serial", Value: record.Serial, Source: "advertisement"},
		{Name: "manufacturer_proto_version", Value: strconv.Itoa(record.ProtoVersion), Source: "advertisement"},
		{Name: "manufacturer_encrypt_type", Value: strconv.Itoa(record.EncryptType), Source: "advertisement"},
		{Name: "manufacturer_active", Value: strconv.FormatBool(record.Active), Source: "advertisement"},
		{Name: "manufacturer_verified", Value: strconv.FormatBool(record.Verified), Source: "advertisement"},
		{Name: "manufacturer_support_verified", Value: strconv.FormatBool(record.SupportVerified), Source: "advertisement"},
		{Name: "manufacturer_support_5g", Value: strconv.FormatBool(record.Support5G), Source: "advertisement"},
	}
}

type secp160r1Curve struct {
	p  *big.Int
	n  *big.Int
	b  *big.Int
	gx *big.Int
	gy *big.Int
}

func newSECP160R1Curve() secp160r1Curve {
	fromHex := func(value string) *big.Int {
		out, ok := new(big.Int).SetString(value, 16)
		if !ok {
			panic("invalid secp160r1 constant")
		}
		return out
	}
	return secp160r1Curve{
		p:  fromHex("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF7FFFFFFF"),
		n:  fromHex("0100000000000000000001F4C8F927AED3CA752257"),
		b:  fromHex("1C97BEFC54BD7A8B65ACF89F81D4D4ADC565FA45"),
		gx: fromHex("4A96B5688EF573284664698968C38BB913CBFC82"),
		gy: fromHex("23A628553168947D59DCC912042351377AC5FB32"),
	}
}

func generateSECP160R1Key(random io.Reader) ([]byte, []byte, error) {
	curve := newSECP160R1Curve()
	one := big.NewInt(1)
	max := new(big.Int).Sub(curve.n, one)
	privateScalar, err := rand.Int(random, max)
	if err != nil {
		return nil, nil, err
	}
	privateScalar.Add(privateScalar, one)
	x, y := curve.scalarMult(curve.gx, curve.gy, privateScalar.Bytes())
	privateBytes := leftPadBytes(privateScalar, 20)
	publicBytes := append(leftPadBytes(x, 20), leftPadBytes(y, 20)...)
	return privateBytes, publicBytes, nil
}

func (c secp160r1Curve) scalarMult(bx *big.Int, by *big.Int, scalar []byte) (*big.Int, *big.Int) {
	var x, y *big.Int
	for _, b := range scalar {
		for bit := 7; bit >= 0; bit-- {
			if x != nil {
				x, y = c.double(x, y)
			}
			if (b>>bit)&1 == 1 {
				if x == nil {
					x, y = new(big.Int).Set(bx), new(big.Int).Set(by)
				} else {
					x, y = c.add(x, y, bx, by)
				}
			}
		}
	}
	if x == nil {
		return big.NewInt(0), big.NewInt(0)
	}
	return x, y
}

func (c secp160r1Curve) isOnCurve(x *big.Int, y *big.Int) bool {
	if x == nil || y == nil || x.Sign() < 0 || y.Sign() < 0 || x.Cmp(c.p) >= 0 || y.Cmp(c.p) >= 0 {
		return false
	}
	left := new(big.Int).Mul(y, y)
	left.Mod(left, c.p)
	right := new(big.Int).Mul(x, x)
	right.Mul(right, x)
	threeX := new(big.Int).Mul(x, big.NewInt(3))
	right.Sub(right, threeX)
	right.Add(right, c.b)
	right.Mod(right, c.p)
	return left.Cmp(right) == 0
}

func (c secp160r1Curve) add(x1 *big.Int, y1 *big.Int, x2 *big.Int, y2 *big.Int) (*big.Int, *big.Int) {
	if x1.Cmp(x2) == 0 {
		sumY := new(big.Int).Add(y1, y2)
		sumY.Mod(sumY, c.p)
		if sumY.Sign() == 0 {
			return nil, nil
		}
		return c.double(x1, y1)
	}
	numerator := new(big.Int).Sub(y2, y1)
	denominator := new(big.Int).Sub(x2, x1)
	denominator.Mod(denominator, c.p)
	denominator.ModInverse(denominator, c.p)
	lambda := new(big.Int).Mul(numerator, denominator)
	lambda.Mod(lambda, c.p)

	x3 := new(big.Int).Mul(lambda, lambda)
	x3.Sub(x3, x1)
	x3.Sub(x3, x2)
	x3.Mod(x3, c.p)

	y3 := new(big.Int).Sub(x1, x3)
	y3.Mul(lambda, y3)
	y3.Sub(y3, y1)
	y3.Mod(y3, c.p)
	return x3, y3
}

func (c secp160r1Curve) double(x1 *big.Int, y1 *big.Int) (*big.Int, *big.Int) {
	if y1.Sign() == 0 {
		return nil, nil
	}
	threeX2 := new(big.Int).Mul(x1, x1)
	threeX2.Mul(threeX2, big.NewInt(3))
	threeX2.Sub(threeX2, big.NewInt(3))
	denominator := new(big.Int).Mul(y1, big.NewInt(2))
	denominator.Mod(denominator, c.p)
	denominator.ModInverse(denominator, c.p)
	lambda := new(big.Int).Mul(threeX2, denominator)
	lambda.Mod(lambda, c.p)

	x3 := new(big.Int).Mul(lambda, lambda)
	x3.Sub(x3, new(big.Int).Mul(x1, big.NewInt(2)))
	x3.Mod(x3, c.p)

	y3 := new(big.Int).Sub(x1, x3)
	y3.Mul(lambda, y3)
	y3.Sub(y3, y1)
	y3.Mod(y3, c.p)
	return x3, y3
}

func leftPadBytes(value *big.Int, size int) []byte {
	out := make([]byte, size)
	valueBytes := value.Bytes()
	copy(out[size-len(valueBytes):], valueBytes)
	return out
}
