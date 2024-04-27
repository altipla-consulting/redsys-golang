package redsys

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignProductionTPVTransactions(t *testing.T) {
	merchant := Merchant{
		Code:            "123456789",
		Name:            "Hotel",
		Terminal:        1,
		Secret:          "sq7HjrUOBfKmC576ILgskD5srU870gJ7",
		URLNotification: "https://notify-url.com",
		Debug:           true,
	}
	session := Session{
		Order:   "code",
		Lang:    LangES,
		Client:  "Name",
		Amount:  12912,
		Product: "Reserva Web",
		URLOK:   "https://url-ok.com",
		URLKO:   "https://urk-ko.com",
	}
	signed, err := Sign(context.Background(), merchant, session)
	require.NoError(t, err)

	require.Equal(t, signed.Endpoint, EndpointDebug)

	require.Equal(t, signed.SignatureVersion, "HMAC_SHA256_V1")
	require.Equal(t, signed.Signature, "jhW+7AYJylFdKnRyiufBdAWZaKbmu9ywAcOwKYkJtvM=")

	decoded, err := base64.StdEncoding.DecodeString(signed.Params)
	require.NoError(t, err)
	params := map[string]interface{}{}
	err = json.Unmarshal(decoded, &params)
	require.NoError(t, err)

	require.Equal(t, params, map[string]interface{}{
		"Ds_Merchant_TransactionType":    float64(0),
		"Ds_Merchant_Order":              "code",
		"Ds_Merchant_MerchantURL":        "https://notify-url.com",
		"Ds_Merchant_ConsumerLanguage":   "001",
		"Ds_Merchant_UrlOK":              "https://url-ok.com",
		"Ds_Merchant_UrlKO":              "https://urk-ko.com",
		"Ds_Merchant_MerchantCode":       "123456789",
		"Ds_Merchant_Amount":             float64(12912),
		"Ds_Merchant_Currency":           float64(978),
		"Ds_Merchant_ProductDescription": "Reserva Web",
		"Ds_Merchant_Titular":            "Name",
		"Ds_Merchant_MerchantName":       "Hotel",
		"Ds_Merchant_Terminal":           float64(1),
	})
}

func TestSign(t *testing.T) {
	merchant := Merchant{
		Code:            "123456789",
		Name:            "Hotel",
		Terminal:        1,
		Secret:          "sq7HjrUOBfKmC576ILgskD5srU870gJ7",
		URLNotification: "https://notify-url.com",
	}
	session := Session{
		Order:   "code",
		Lang:    LangES,
		Client:  "Name",
		Amount:  12912,
		Product: "Reserva Web",
		URLOK:   "https://url-ok.com",
		URLKO:   "https://urk-ko.com",
	}
	_, err := Sign(context.Background(), merchant, session)
	require.NoError(t, err)
}

func TestSignRetried(t *testing.T) {
	merchant := Merchant{
		Secret: "sq7HjrUOBfKmC576ILgskD5srU870gJ7",
	}
	session := Session{
		Order: "foo-code-bar",
		Lang:  LangES,
	}
	signed, err := Sign(context.Background(), merchant, session)
	require.NoError(t, err)

	decoded, err := base64.StdEncoding.DecodeString(signed.Params)
	require.NoError(t, err)
	params := map[string]interface{}{}
	err = json.Unmarshal(decoded, &params)
	require.NoError(t, err)

	require.Equal(t, params["Ds_Merchant_Order"], "foo-code-bar")
}

func TestSignCutsLongNames(t *testing.T) {
	merchant := Merchant{
		Secret: "sq7HjrUOBfKmC576ILgskD5srU870gJ7",
	}
	session := Session{
		Order:  "foo-code-bar",
		Lang:   LangES,
		Client: "123456789012345678901234567890123456789012345678901234567890 more than 60 chars",
	}
	signed, err := Sign(context.Background(), merchant, session)
	require.NoError(t, err)

	decoded, err := base64.StdEncoding.DecodeString(signed.Params)
	require.NoError(t, err)
	params := map[string]interface{}{}
	err = json.Unmarshal(decoded, &params)
	require.NoError(t, err)

	require.Equal(t, params["Ds_Merchant_Titular"], "12345678901234567890123456789012345678901234567890123456789")
}

func TestSignatureVersionUnknown(t *testing.T) {
	_, err := ParseParams(Signed{SignatureVersion: "foo"})
	require.EqualError(t, err, "unknown signature version: foo")
}

func TestParseParams(t *testing.T) {
	params := `{"Ds_Order": "order-code"}`
	signed := Signed{
		SignatureVersion: "HMAC_SHA256_V1",
		Params:           base64.StdEncoding.EncodeToString([]byte(params)),
	}
	_, err := ParseParams(signed)
	require.NoError(t, err)
}

func TestParseParamsReadsNumericResponse(t *testing.T) {
	paramsEncoded := `{"Ds_Order": "order-code", "Ds_Response": "099"}`
	signed := Signed{
		SignatureVersion: "HMAC_SHA256_V1",
		Params:           base64.StdEncoding.EncodeToString([]byte(paramsEncoded)),
	}
	params, err := ParseParams(signed)
	require.NoError(t, err)

	require.EqualValues(t, params.Response, 99)
}

func TestConfirmBadSignature(t *testing.T) {
	params := `{"Ds_Order": "00order-code"}`
	signed := Signed{
		SignatureVersion: "HMAC_SHA256_V1",
		Signature:        "foobarqu",
		Params:           base64.StdEncoding.EncodeToString([]byte(params)),
	}
	_, err := Confirm(context.Background(), "sq7HjrUOBfKmC576ILgskD5srU870gJ7", signed)
	require.EqualError(t, err, `bad signature, got "foobarqu" expected "cQ3etTziVy1Dzvs72w9KS8vDALtU0EPiDm0rWvb7mBU="`)
}

func TestConfirmCancellations(t *testing.T) {
	params := `{"Ds_Order": "00order-code", "Ds_Response": "9915", "Ds_Date": "24/11/2021", "Ds_Hour": "08:00"}`
	signed := Signed{
		SignatureVersion: "HMAC_SHA256_V1",
		Signature:        "9X6rL8CmYglpb3CmXFR_8YFDAbEuvQ9YK-wA0yuuhFQ=",
		Params:           base64.StdEncoding.EncodeToString([]byte(params)),
	}
	operation, err := Confirm(context.Background(), "sq7HjrUOBfKmC576ILgskD5srU870gJ7", signed)
	require.NoError(t, err)

	require.Equal(t, operation.Status, StatusCancelled)
}

func TestConfirmRepeatedTransaction(t *testing.T) {
	params := `{"Ds_Order": "00order-code", "Ds_Response": "0913", "Ds_Date": "24/11/2021", "Ds_Hour": "08:00"}`
	signed := Signed{
		SignatureVersion: "HMAC_SHA256_V1",
		Signature:        "_vMwdgTbkrldjxmz5e1xOgfXx42gLkwe__CD6jOWBX0=",
		Params:           base64.StdEncoding.EncodeToString([]byte(params)),
	}
	operation, err := Confirm(context.Background(), "sq7HjrUOBfKmC576ILgskD5srU870gJ7", signed)
	require.NoError(t, err)

	require.Equal(t, operation.Status, StatusRepeated)
}

func TestConfirm(t *testing.T) {
	params := `{"Ds_Order": "00order-code", "Ds_Response": "0", "Ds_Date": "24/11/2021", "Ds_Hour": "08:00", "Ds_Card_Country": "SPAIN", "Ds_AuthorisationCode": "123456", "Ds_Card_Type": "C"}`
	signed := Signed{
		SignatureVersion: "HMAC_SHA256_V1",
		Signature:        "KKr4Cjwr2w94_nkHMU7ijkHWiTHMrJm84Iho2eSlXlA=",
		Params:           base64.StdEncoding.EncodeToString([]byte(params)),
	}
	operation, err := Confirm(context.Background(), "sq7HjrUOBfKmC576ILgskD5srU870gJ7", signed)
	require.NoError(t, err)

	require.Equal(t, operation.Status, StatusApproved)
	require.Equal(t, operation.Params.Country, "SPAIN")
	require.Equal(t, operation.Params.AuthCode, "123456")
	require.Equal(t, operation.Params.CardType, "C")
	require.EqualValues(t, operation.Sent.Day(), 24)
	require.EqualValues(t, operation.Sent.Month(), 11)
	require.EqualValues(t, operation.Sent.Year(), 2021)
	require.EqualValues(t, operation.Sent.Hour(), 8)
	require.EqualValues(t, operation.Sent.Minute(), 0)
}
