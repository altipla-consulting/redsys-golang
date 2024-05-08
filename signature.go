package redsys

import (
	"context"
	"crypto/cipher"
	"crypto/des"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"time"
)

const (
	EndpointProduction = "https://sis.redsys.es/sis/realizarPago"
	EndpointDebug      = "https://sis-t.redsys.es:25443/sis/realizarPago"
)

// Signed TPV transaction to send to the bank.
type Signed struct {
	// Signature of the parameters.
	Signature string

	// Version of the signature.
	SignatureVersion string

	// Params to send.
	Params string

	// Output only. It will return the endpoint of the call.
	Endpoint string
}

// Merchant data
type Merchant struct {
	// Merchant code assigned by the bank.
	Code string

	// Merchant name to show in the receipt.
	Name string

	// Terminal number assigned by the bank.
	Terminal int64

	// Secret to sign transactions assigned by the bank.
	Secret string

	// URL where the notification will be sent.
	URLNotification string

	// Enable debug endpoint.
	Debug bool
}

// Session data
type Session struct {
	// Code of the session. It should have 4 digits and 8 characters.
	Order string

	// Two-letters code of the language. Use English if unknown, please.
	Lang Lang

	// Name of the client to show in the receipt.
	Client string

	// Amount in cents to pay.
	Amount int32

	// Product name to show in the receipt.
	Product string

	// URL to return to when the transaction is approved.
	URLOK string

	// URL to return to when the transaction is cancelled.
	URLKO string

	// Raw data that will be sent back in the confirmation.
	Data string
}

type TransactionType int64

const (
	TransactionTypeSimpleAuthorization = TransactionType(0)
)

type Currency int64

const (
	CurrencyEuros = Currency(978)
)

type Lang string

const (
	LangES = Lang("001")
	LangEN = Lang("002")
	LangCA = Lang("003")
	LangFR = Lang("004")
	LangDE = Lang("005")
	LangIT = Lang("007")
	LangPT = Lang("009")
)

type tpvRequest struct {
	MerchantCode    string          `json:"Ds_Merchant_MerchantCode"`
	Terminal        int64           `json:"Ds_Merchant_Terminal"`
	TransactionType TransactionType `json:"Ds_Merchant_TransactionType"`
	Amount          int32           `json:"Ds_Merchant_Amount"`
	Currency        Currency        `json:"Ds_Merchant_Currency"`
	Order           string          `json:"Ds_Merchant_Order"`
	MerchantURL     string          `json:"Ds_Merchant_MerchantURL"`
	Product         string          `json:"Ds_Merchant_ProductDescription"`
	Client          string          `json:"Ds_Merchant_Titular"`
	Lang            Lang            `json:"Ds_Merchant_ConsumerLanguage"`
	URLOK           string          `json:"Ds_Merchant_UrlOK"`
	URLKO           string          `json:"Ds_Merchant_UrlKO"`
	MerchantName    string          `json:"Ds_Merchant_MerchantName"`
	Data            string          `json:"Ds_Merchant_MerchantData,omitempty"`
}

func Sign(ctx context.Context, merchant Merchant, session Session) (Signed, error) {
	if len(session.Client) > 59 {
		session.Client = session.Client[:59]
	}
	params := tpvRequest{
		MerchantCode:    merchant.Code,
		Terminal:        merchant.Terminal,
		TransactionType: TransactionTypeSimpleAuthorization,
		Amount:          session.Amount,
		Currency:        CurrencyEuros,
		Order:           session.Order,
		MerchantURL:     merchant.URLNotification,
		Product:         session.Product,
		Client:          session.Client,
		Lang:            session.Lang,
		URLOK:           session.URLOK,
		URLKO:           session.URLKO,
		MerchantName:    merchant.Name,
		Data:            session.Data,
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return Signed{}, fmt.Errorf("cannot marshal params: %v", err)
	}
	paramsStr := base64.StdEncoding.EncodeToString(paramsJSON)

	signature, err := sign(merchant.Secret, params.Order, paramsStr)
	if err != nil {
		return Signed{}, fmt.Errorf("%v", err)
	}
	signed := Signed{
		Signature:        base64.StdEncoding.EncodeToString(signature),
		SignatureVersion: "HMAC_SHA256_V1",
		Params:           paramsStr,
		Endpoint:         EndpointProduction,
	}
	if merchant.Debug {
		signed.Endpoint = EndpointDebug
	}
	return signed, nil
}

type Status string

const (
	StatusUnknown   = Status("")
	StatusApproved  = Status("approved")
	StatusCancelled = Status("cancelled")
	StatusRepeated  = Status("repeated")
)

type Operation struct {
	Status       Status
	Sent         time.Time
	Params       Params
	IsCreditCard bool

	// Raw response code of the bank.
	ResponseCode int64
}

type Params struct {
	Response int64  `json:"-"`
	Order    string `json:"Ds_Order"`
	Date     string `json:"Ds_Date"`
	Time     string `json:"Ds_Hour"`
	Country  string `json:"Ds_Card_Country"`
	AuthCode string `json:"Ds_AuthorisationCode"`
	CardType string `json:"Ds_Card_Type"`
	Data     string `json:"Ds_MerchantData"`

	RawResponse string `json:"Ds_Response"`
}

func ParseParams(signed Signed) (Params, error) {
	if signed.SignatureVersion != "HMAC_SHA256_V1" {
		return Params{}, fmt.Errorf("unknown signature version: %s", signed.SignatureVersion)
	}
	decoded, err := base64.StdEncoding.DecodeString(signed.Params)
	if err != nil {
		return Params{}, fmt.Errorf("cannot decode params: %v", err)
	}
	params := Params{}
	if err = json.Unmarshal(decoded, &params); err != nil {
		return Params{}, fmt.Errorf("cannot unmarshal params: %v", err)
	}
	if params.RawResponse != "" {
		params.Response, err = strconv.ParseInt(params.RawResponse, 10, 64)
		if err != nil {
			return Params{}, fmt.Errorf("cannot parse response %q: %v", params.RawResponse, err)
		}
	}

	params.Data, err = url.QueryUnescape(params.Data)
	if err != nil {
		return Params{}, fmt.Errorf("cannot unescape data %q: %v", params.Data, err)
	}

	return params, nil
}

func Confirm(ctx context.Context, secret string, signed Signed) (Operation, error) {
	params, err := ParseParams(signed)
	if err != nil {
		return Operation{}, fmt.Errorf("cannot parse params: %v", err)
	}

	signature, err := sign(secret, params.Order, signed.Params)
	if err != nil {
		return Operation{}, fmt.Errorf("%v", err)
	}

	decodedSignature, err := base64.URLEncoding.DecodeString(signed.Signature)
	if err != nil {
		return Operation{}, fmt.Errorf("cannot decode signature: %v", err)
	}
	if !hmac.Equal(signature, decodedSignature) {
		return Operation{}, fmt.Errorf("bad signature, got %q expected %q", signed.Signature, base64.URLEncoding.EncodeToString(signature))
	}

	operation := Operation{
		Params:       params,
		ResponseCode: params.Response,
	}

	dt, err := url.QueryUnescape(fmt.Sprintf("%s %s", params.Date, params.Time))
	if err != nil {
		return Operation{}, fmt.Errorf("cannot unescape datetime %q %q: %v", params.Date, params.Time, err)
	}
	operation.Sent, err = time.Parse("02/01/2006 15:04", dt)
	if err != nil {
		return Operation{}, fmt.Errorf("failed to parse datetime %q: %v", dt, err)
	}

	badData := []int64{
		101,  // Expired card
		129,  // Wrong CVV
		180,  // Alien card service
		184,  // Error with the owner authentication
		191,  // Wrong expiration date
		9142, // Excess time for payment
	}
	switch {
	case params.Response == 9915:
		operation.Status = StatusCancelled

	case params.Response == 913:
		operation.Status = StatusRepeated

	case slices.Contains(badData, params.Response):
		operation.Status = StatusCancelled

	case params.Response >= 0 && params.Response <= 99:
		operation.Status = StatusApproved
		operation.IsCreditCard = (params.CardType == "C")
	}

	return operation, nil
}

func sign(secret, order, content string) ([]byte, error) {
	decodedSecret, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return nil, fmt.Errorf("cannot decode secret: %v", err)
	}
	block, err := des.NewTripleDESCipher(decodedSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cipher: %v", err)
	}

	// Zeros IV obtained from the official implementation in PHP.
	mode := cipher.NewCBCEncrypter(block, []byte("\x00\x00\x00\x00\x00\x00\x00\x00"))

	key := make([]byte, 16)
	mode.CryptBlocks(key, []byte(fmt.Sprintf("%s\x00\x00\x00\x00", order)))

	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(content))
	return mac.Sum(nil), nil
}
