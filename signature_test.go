package tpv

// import (
// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"

// 	"encoding/base64"
// 	"encoding/json"
// 	"time"

// 	"github.com/juju/errors"
// 	"golang.org/x/net/context"
// )

// var _ = Describe("Sign", func() {
// 	It("Should sign TPV transactions", func() {
// 		merchant := Merchant{
// 			Code:            "123456789",
// 			Name:            "Foo Hotels",
// 			Terminal:        1,
// 			Secret:          "XXX",
// 			URLNotification: "https://panel.altiplaconsulting.net/_/onetbooking/book/tpv/confirmation",
// 		}
// 		session := Session{
// 			Code:    "foo-code-bar",
// 			Lang:    LangES,
// 			Client:  "Foo name",
// 			Amount:  12912,
// 			Product: "Reserva Web",
// 			URLOK:   "https://example.com/booking/tpv-ok?booking=foo-code",
// 			URLKO:   "https://example.com/booking/tpv-ko?booking=foo-code",
// 		}
// 		signed, err := Sign(context.Background(), merchant, session)
// 		Expect(err).To(Succeed())

// 		Expect(signed.SignatureVersion).To(Equal("HMAC_SHA256_V1"))
// 		Expect(signed.Signature).To(Equal("rlTJH+KV407EuGVMd+xKMMGeeqWzLmrRUhFsVQkUbsM="))

// 		decoded, err := base64.StdEncoding.DecodeString(signed.Params)
// 		Expect(err).To(Succeed())
// 		params := map[string]interface{}{}
// 		err = json.Unmarshal(decoded, &params)
// 		Expect(err).To(Succeed())

// 		Expect(params).To(Equal(map[string]interface{}{
// 			"Ds_Merchant_TransactionType":    float64(0),
// 			"Ds_Merchant_Order":              "0000foo-code",
// 			"Ds_Merchant_MerchantURL":        "https://panel.altiplaconsulting.net/_/onetbooking/book/tpv/confirmation",
// 			"Ds_Merchant_ConsumerLanguage":   "001",
// 			"Ds_Merchant_UrlOK":              "https://example.com/booking/tpv-ok?booking=foo-code",
// 			"Ds_Merchant_UrlKO":              "https://example.com/booking/tpv-ko?booking=foo-code",
// 			"Ds_Merchant_MerchantCode":       "123456789",
// 			"Ds_Merchant_Amount":             float64(12912),
// 			"Ds_Merchant_Currency":           float64(978),
// 			"Ds_Merchant_ProductDescription": "Reserva Web",
// 			"Ds_Merchant_Titular":            "Foo name",
// 			"Ds_Merchant_MerchantName":       "Foo Hotels",
// 			"Ds_Merchant_Terminal":           float64(1),
// 		}))
// 	})

// 	It("Should sign testing TPV transactions", func() {
// 		merchant := Merchant{
// 			Code:            TestingMerchantCode,
// 			Name:            TestingMerchantName,
// 			Terminal:        TestingTerminal,
// 			Secret:          TestingSecret,
// 			URLNotification: "https://panel.altiplaconsulting.net/_/onetbooking/book/tpv/confirmation",
// 		}
// 		session := Session{
// 			Code:    "foo-code-bar",
// 			Lang:    LangES,
// 			Client:  "Foo name",
// 			Amount:  12912,
// 			Product: "Reserva Web",
// 			URLOK:   "https://example.com/booking/tpv-ok?booking=foo-code",
// 			URLKO:   "https://example.com/booking/tpv-ko?booking=foo-code",
// 		}
// 		_, err := Sign(context.Background(), merchant, session)
// 		Expect(err).To(Succeed())
// 	})

// 	It("Should generate retried transactions", func() {
// 		merchant := Merchant{
// 			Secret: "XXX",
// 		}
// 		session := Session{
// 			Code: "foo-code-bar",
// 			Lang: LangES,
// 			// Retry: 3,
// 		}
// 		signed, err := Sign(context.Background(), merchant, session)
// 		Expect(err).To(Succeed())

// 		decoded, err := base64.StdEncoding.DecodeString(signed.Params)
// 		Expect(err).To(Succeed())
// 		params := map[string]interface{}{}
// 		err = json.Unmarshal(decoded, &params)
// 		Expect(err).To(Succeed())

// 		Expect(params["Ds_Merchant_Order"]).To(Equal("0003foo-code"))
// 	})

// 	It("Should cut long client names", func() {
// 		merchant := Merchant{
// 			Secret: "XXX",
// 		}
// 		session := Session{
// 			Code: "foo-code-bar",
// 			Lang: LangES,
// 			// Retry:  3,
// 			Client: "123456789012345678901234567890123456789012345678901234567890 more than 60 chars",
// 		}
// 		signed, err := Sign(context.Background(), merchant, session)
// 		Expect(err).To(Succeed())

// 		decoded, err := base64.StdEncoding.DecodeString(signed.Params)
// 		Expect(err).To(Succeed())
// 		params := map[string]interface{}{}
// 		err = json.Unmarshal(decoded, &params)
// 		Expect(err).To(Succeed())

// 		Expect(params["Ds_Merchant_Titular"]).To(Equal("12345678901234567890123456789012345678901234567890123456789"))
// 	})
// })

// var _ = Describe("ParseParams", func() {
// 	It("Should reject unknown signature versions", func() {
// 		_, err := ParseParams(Signed{SignatureVersion: "foo"})
// 		Expect(err).To(MatchError("unknown signature version: foo"))
// 	})

// 	It("Should parse the raw confirmation", func() {
// 		params := `{"Ds_Order": "0000foo-code"}`
// 		signed := Signed{
// 			SignatureVersion: "HMAC_SHA256_V1",
// 			Params:           base64.StdEncoding.EncodeToString([]byte(params)),
// 		}
// 		_, err := ParseParams(signed)
// 		Expect(err).To(Succeed())
// 	})

// 	It("Should parse response number after reading the JSON", func() {
// 		paramsEncoded := `{"Ds_Order": "0000foo-code", "Ds_Response": "099"}`
// 		signed := Signed{
// 			SignatureVersion: "HMAC_SHA256_V1",
// 			Params:           base64.StdEncoding.EncodeToString([]byte(paramsEncoded)),
// 		}
// 		params, err := ParseParams(signed)
// 		Expect(err).To(Succeed())
// 		Expect(params.Response).To(BeEquivalentTo(99))
// 	})
// })

// var _ = Describe("Confirm", func() {
// 	It("Should reject unknown signature versions", func() {
// 		_, err := Confirm(context.Background(), "", Signed{SignatureVersion: "foo"})
// 		Expect(err).To(MatchError("unknown signature version: foo"))
// 	})

// 	It("Should reject bad signatures", func() {
// 		params := `{"Ds_Order": "0000foo-code"}`
// 		signed := Signed{
// 			SignatureVersion: "HMAC_SHA256_V1",
// 			Signature:        "foobarqu",
// 			Params:           base64.StdEncoding.EncodeToString([]byte(params)),
// 		}
// 		_, err := Confirm(context.Background(), "XXX", signed)
// 		Expect(errors.Cause(err)).To(MatchError("bad signature, expected: YYY"))
// 	})

// 	It("Should detect cancellations", func() {
// 		params := `{"Ds_Order": "0000foo-code", "Ds_Response": "9915"}`
// 		signed := Signed{
// 			SignatureVersion: "HMAC_SHA256_V1",
// 			Signature:        "KsC18N4ThFt31HYWWVJuCujdrgILNUk-IqYVis_L4BU=",
// 			Params:           base64.StdEncoding.EncodeToString([]byte(params)),
// 		}
// 		operation, err := Confirm(context.Background(), "XXX", signed)
// 		Expect(err).To(Succeed())

// 		Expect(operation.Status).To(Equal(StatusCancelled))
// 	})

// 	It("Should detect repeated codes", func() {
// 		params := `{"Ds_Order": "0000foo-code", "Ds_Response": "0913"}`
// 		signed := Signed{
// 			SignatureVersion: "HMAC_SHA256_V1",
// 			Signature:        "XSk4b_tDuJLECQDGLf6KNMhB4Yug9F_NPaI49CQQzNg=",
// 			Params:           base64.StdEncoding.EncodeToString([]byte(params)),
// 		}
// 		operation, err := Confirm(context.Background(), "XXX", signed)
// 		Expect(err).To(Succeed())

// 		Expect(operation.Status).To(Equal(StatusRepeated))
// 	})

// 	It("Should detect payments", func() {
// 		params := `{"Ds_Order": "0000foo-code", "Ds_Response": "0", "Ds_Date": "10/04/2016", "Ds_Hour": "11:40", "Ds_Card_Country": "SPAIN", "Ds_AuthorisationCode": "123456", "Ds_Card_Type": "C"}`
// 		signed := Signed{
// 			SignatureVersion: "HMAC_SHA256_V1",
// 			Signature:        "S-NH4GI-PLw6ykg7MAujFohZeZmaFR_AIwvD8i892w8=",
// 			Params:           base64.StdEncoding.EncodeToString([]byte(params)),
// 		}
// 		operation, err := Confirm(context.Background(), "XXX", signed)
// 		Expect(err).To(Succeed())

// 		Expect(operation.Status).To(Equal(StatusApproved))
// 		Expect(operation.Params.Country).To(Equal("SPAIN"))
// 		Expect(operation.Params.AuthCode).To(Equal("123456"))
// 		Expect(operation.Params.CardType).To(BeTrue())
// 		Expect(operation.Sent).To(Equal(time.Date(2016, time.April, 10, 11, 40, 0, 0, time.UTC)))
// 	})

// 	It("Should process encoded dates", func() {
// 		params := `{"Ds_Order": "0000foo-code", "Ds_Response": "0", "Ds_Date": "10%2F04%2F2016", "Ds_Hour": "11:40"}`
// 		signed := Signed{
// 			SignatureVersion: "HMAC_SHA256_V1",
// 			Signature:        "VFLFdGhhqPCXGn-clDWZPV-u0fZAHsD9EYI4iavM71E=",
// 			Params:           base64.StdEncoding.EncodeToString([]byte(params)),
// 		}
// 		operation, err := Confirm(context.Background(), "XXX", signed)
// 		Expect(err).To(Succeed())

// 		Expect(operation.Sent).To(Equal(time.Date(2016, time.April, 10, 11, 40, 0, 0, time.UTC)))
// 	})
// })
