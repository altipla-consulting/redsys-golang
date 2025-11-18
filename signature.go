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
	"regexp"
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
	// Signature of the parameters. Assign to Ds_Signature.
	Signature string

	// Version of the signature. Assign to Ds_SignatureVersion.
	SignatureVersion string

	// Params to send. Assign to Ds_MerchantParameters.
	Params string

	// Output only. It will return the endpoint of the call where the info should be sent.
	Endpoint string
}

// Merchant data provided by the bank itself during the integration.
type Merchant struct {
	// Merchant code assigned by the bank.
	Code string

	// Merchant name to show in the receipt. You can freely use any appropiate name.
	Name string

	// Terminal number assigned by the bank.
	Terminal int64

	// Secret to sign transactions assigned by the bank.
	Secret string

	// URL where the asynchronous background notification will be sent.
	URLNotification string

	// Send the data to the test endpoint of the bank.
	Debug bool
}

// Session data that changes for each payment the merchant wants to make.
type Session struct {
	// Code of the session. It should have 4 digits and 8 characters. It should be unique for each retry of the payment.
	Order string

	// Language code. Use English if unknown.
	Lang Lang

	// Name of the client to show in the receipt. Use any appropiate info available.
	Client string

	// Amount in cents to pay.
	Amount int32

	// Product name to show in the receipt.
	Product string

	// URL to return the user to when the transaction is approved.
	URLOK string

	// URL to return the user to when the transaction is cancelled.
	URLKO string

	// Raw custom data that will be sent back in the confirmation when the transaction finishes. You can use it to store
	// identifiers or any other data that facilitates the verification afterwards.
	Data string

	// Payment method to use. By default it will be credit card if empty.
	PaymentMethod PaymentMethod

	// Transaction type to use. By default it will be simple authorization.
	TransactionType TransactionType
}

type TransactionType int64

const (
	TransactionTypeSimpleAuthorization = TransactionType(0)
	TransactionTypePreAuthorization    = TransactionType(1)
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

type PaymentMethod string

const (
	PaymentMethodCreditCard = PaymentMethod("C")
	PaymentMethodBizum      = PaymentMethod("z")
	PaymentMethodPaypal     = PaymentMethod("P")
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
	PaymentMethod   PaymentMethod   `json:"Ds_Merchant_PayMethods,omitempty"`
}

var reOrder = regexp.MustCompile(`^[0-9]{4}[0-9A-Za-z]{8}$`)

// Sign takes all the input data and returns the parameters to be sent to the bank.
func Sign(ctx context.Context, merchant Merchant, session Session) (Signed, error) {
	if !reOrder.MatchString(session.Order) {
		return Signed{}, fmt.Errorf("invalid order format %q", session.Order)
	}
	if len(session.Client) > 59 {
		session.Client = session.Client[:59]
	}

	params := tpvRequest{
		MerchantCode:    merchant.Code,
		Terminal:        merchant.Terminal,
		TransactionType: session.TransactionType,
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
		PaymentMethod:   session.PaymentMethod,
	}
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return Signed{}, fmt.Errorf("cannot marshal params: %v", err)
	}
	paramsStr := base64.URLEncoding.EncodeToString(paramsJSON)

	signature, err := sign(merchant.Secret, params.Order, paramsStr)
	if err != nil {
		return Signed{}, fmt.Errorf("%v", err)
	}
	signed := Signed{
		Signature:        base64.URLEncoding.EncodeToString(signature),
		SignatureVersion: "HMAC_SHA256_V1",
		Params:           paramsStr,
		Endpoint:         EndpointProduction,
	}
	if merchant.Debug {
		signed.Endpoint = EndpointDebug
	}
	return signed, nil
}

// Parsed parameters of the transaction.
type Params struct {
	// Response code of the bank.
	Response int64 `json:"-"`

	// Order code of the transaction.
	Order string `json:"Ds_Order"`

	// Original order code as a string that sometimes comes back empty.
	RawResponse string `json:"Ds_Response"`

	// Date of the transaction in the format "02/01/2006".
	Date string `json:"Ds_Date"`

	// Hour of the transaction in the format "15:04".
	Time string `json:"Ds_Hour"`

	// Country code of the card.
	Country string `json:"Ds_Card_Country"`

	// Authorization code of the card. Should be stored to have a reference to the transaction.
	AuthCode string `json:"Ds_AuthorisationCode"`

	// Card type: MasterCard, Visa, etc.
	CardType string `json:"Ds_Card_Type"`

	// Custom data previously sent that comes back in the confirmation.
	Data string `json:"Ds_MerchantData"`
}

// ParseParams reads the response from the bank and returns the parsed parameters if the signature is valid. If an error
// is returned the input data is compromised and should not be used, the returned params will also be empty.
func ParseParams(signed Signed) (Params, error) {
	if signed.SignatureVersion != "HMAC_SHA256_V1" {
		return Params{}, fmt.Errorf("unknown signature version: %s", signed.SignatureVersion)
	}
	decoded, err := base64.URLEncoding.DecodeString(signed.Params)
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

// Status of a finished transaction.
type Status string

const (
	// StatusUnknown means the transaction has not been correctly detected. You can error out as a cancellation
	// or as an error depending on the context.
	StatusUnknown = Status("")

	// StatusApproved means the transaction was approved by the bank and the money was transferred.
	StatusApproved = Status("approved")

	// StatusCancelled means the transaction was cancelled by the user.
	StatusCancelled = Status("cancelled")

	// StatusRepeated means the transaction has been sent repeatedly to the bank. It is a programming error that should
	// not happen if a different Order code is used for each retry.
	StatusRepeated = Status("repeated")
)

// Operation represents the result of a payment operation.
type Operation struct {
	// Status of the operation.
	Status Status

	// Sent date of the operation.
	Sent time.Time

	// All the parsed parameters of the transaction.
	Params Params

	// True if the transaction was a credit card payment.
	IsCreditCard bool

	// Raw response code of the bank.
	ResponseCode int64
}

// Confirm reads the response from the bank and parses the response to determine the status of the transaction in a more
// easy to use way. If an error is returned the input data is compromised and should not be used, the returned operation
// will also be empty.
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

	cancelled := []int64{
		101,  // Tarjeta caducada, no reintentar la operación.
		104,  // Operación no permitida para esa tarjeta, consulte con la entidad emisora de la misma.
		129,  // Código de seguridad (CVV2/CVC2) incorrecto.
		180,  // Tarjeta ajena al servicio.
		190,  // Denegación del emisor sin especificar motivo.
		184,  // Error en la autenticación del titular.
		191,  // Fecha de caducidad errónea.
		9080, // Error genérico. Consulte con Soporte.
		9142, // Tiempo excecido para el pago.
		9221, // El CVV2 es obligatorio.
		9593, // Error en la operacion de autenticacion EMV3DS,el transStatus de la consulta final de la operación no está definido.
		9602, // Error en el proceso de autenticación 3DSecure v2 – Respuesta Areq U.
		9673, // Operación cancelada. El usuario no desea seguir.
		9754, // La tarjeta no permite autenticación en versión 2.
		9915, // A petición del usuario se ha cancelado el pago.
		9589, // Operacion de autenticacion EMV3DS rechazada, respuesta sin CRes.
		9590, // Operacion de autenticacion EMV3DS rechazada, error al desmontar la respuesta CRes.
		9601, // El banco emisor indica que no es posible autenticar la tarjeta – Respuesta Areq R.
	}
	switch {
	case params.Response == 913 || params.Response == 9051:
		operation.Status = StatusRepeated

	case slices.Contains(cancelled, params.Response):
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
