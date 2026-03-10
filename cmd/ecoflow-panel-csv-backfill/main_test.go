package main

import "testing"

func TestBackfillRowFillsDerivedFields(t *testing.T) {
	header := []string{
		"Voc_V", "Vmp_V", "Imp_A", "Isc_A", "TempCoeff_Voc_%/C",
		"Voc_temp_coeff_missing", "Voc_0C_V", "Voc_-20C_V", "Voc_-25C_V",
		"Voc_safety_basis", "Voc_safety_V", "MaxSeries_60V", "EcoFlow_compatibility",
	}
	idx := mapHeaders(header)
	row := make([]string, len(header))
	row[idx["Voc_V"]] = "52"
	row[idx["Vmp_V"]] = "44"
	row[idx["Imp_A"]] = "10"

	if !backfillRow(row, idx) {
		t.Fatal("expected row to change")
	}
	if row[idx["Voc_temp_coeff_missing"]] != "True" {
		t.Fatalf("expected temp coeff missing flag, got=%q", row[idx["Voc_temp_coeff_missing"]])
	}
	if row[idx["Voc_safety_basis"]] == "" || row[idx["Voc_safety_V"]] == "" || row[idx["MaxSeries_60V"]] == "" {
		t.Fatalf("expected derived voltage fields to be populated: %#v", row)
	}
	if row[idx["EcoFlow_compatibility"]] == "" {
		t.Fatal("expected compatibility string")
	}
}
