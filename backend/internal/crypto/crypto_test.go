package crypto

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	svc, err := New("test-master-key-1234567890123456")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	plain := "SIAT_TOKEN_DELEGADO_SUPER_SECRETO"
	enc, err := svc.EncryptString(plain)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if enc == plain {
		t.Fatalf("ciphertext no debe ser igual al plaintext")
	}
	dec, err := svc.DecryptString(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if dec != plain {
		t.Fatalf("roundtrip mismatch: got %q want %q", dec, plain)
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	svc1 := MustNew("key-one-1234567890123456789012")
	svc2 := MustNew("key-two-1234567890123456789012")
	enc, _ := svc1.EncryptString("secret")
	if _, err := svc2.DecryptString(enc); err == nil {
		t.Fatalf("decrypt con clave distinta debe fallar")
	}
}

func TestEncryptEmptyFails(t *testing.T) {
	svc := MustNew("test-master-key-1234567890123456")
	if _, err := svc.Encrypt([]byte{}); err == nil {
		t.Fatalf("encrypt vacío debe fallar")
	}
}
