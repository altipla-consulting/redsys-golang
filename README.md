# redsys-golang

[![Go Reference](https://pkg.go.dev/badge/github.com/altipla-consulting/redsys-golang.svg)](https://pkg.go.dev/github.com/altipla-consulting/redsys-golang)

Redsys integration for Go.


## Install

```shell
go get github.com/altipla-consulting/redsys-golang
```

## Usage

Follow this steps to integrate with Redsys:

1. First and foremost you will need the bank credentials that should be requested in person at your entity. The will provide you with access to the test environment and with the following important information:

    - Merchant code.
    - Terminal number.
    - A signing key.

2. Sign the payment server side with this library:

    ```go
    sess := redsys.Session{
      Order:  "0001abcdabcd",
      Lang:   redsys.LangES,
      Client: "John Doe",
      Amount: 1234,
      Product: "My Awesome Product",
      URLOK:   "https://www.example.com/order-confirmed",
      URLKO:   "https://www.example.com/order-cancelled",
      Data:    "custom-data",
    }
    merchant := redsys.Merchant{
      Code:     "1234_YOUR_MERCHANT_CODE",
      Name:     "My Awesome Shop",
      Terminal: 1234,
      Secret:   "YOUR_SECRET",
      URLNotification: "https://www.example.com/background-notification",
      Debug:    true,
    }
    signed, err := redsys.Sign(ctx, merchant, rs)
    if err != nil {
      return nil, errors.Trace(err)
    }
    ```

3. Send from the browser a form to the bank. It is important to send it client side, it can't be a POST request from Go.

    You can prepare a HTML form:

    ```html
    <form method="POST" action="{{.Signed.Endpoint}}">
      <input type="hidden" name="Ds_SignatureVersion" value="{{.Signed.SignatureVersion}}">
      <input type="hidden" name="Ds_Signature" value="{{.Signed.Signature}}">
      <input type="hidden" name="Ds_MerchantParameters" value="{{.Signed.Params}}">

      <button type="submit">Submit</button>
    </form>
    ```

    If you are in a more dynamic environment (e.g. with React or Vue using APIs to sign the transaction) you can also use a pure Javascript solution to build and send the form:

    ```js
    // DOM manipulation to create a fake form that we can send to the
    // remote POST endpoint of the bank network.
    let form = document.createElement('form')
    form.setAttribute('method', 'POST')
    form.setAttribute('action', params.url)

    let input = document.createElement('input')
    input.setAttribute('type', 'hidden')
    input.setAttribute('name', 'Ds_SignatureVersion')
    input.value = params.signatureVersion
    form.appendChild(input)

    input = document.createElement('input')
    input.setAttribute('type', 'hidden')
    input.setAttribute('name', 'Ds_Signature')
    input.value = params.signature
    form.appendChild(input)

    input = document.createElement('input')
    input.setAttribute('type', 'hidden')
    input.setAttribute('name', 'Ds_MerchantParameters')
    input.value = params.params
    form.appendChild(input)
    document.body.appendChild(form)

    form.submit()
    ```

4. The user will pay, cancel, verify its credit card, etc. inside the bank page. When the transaction finishes (successfully or not) a background notification will be sent automatically to the configured endpoint. In parallel the user will return to the OK/KO page. Both requests will have parameters in the query string that we can read.

5. Verify the received parameters. Anyone can send requests to public pages so you need to be completely sure the bank has signed the data and everything is legal and secure. Use our library to verify the parameters:

    ```go
    signed := redsys.Signed{
      SignatureVersion: r.FormValue("Ds_SignatureVersion"),
      Params:           r.FormValue("Ds_MerchantParameters"),
      Signature:        r.FormValue("Ds_Signature"),
    }
    operation, err := redsys.Confirm(ctx, "YOUR_SECRET", signed)
    if err != nil {
      return nil, errors.Trace(err)
    }    
    ```

6. Use the `operation` variable to show messages to the user, approve the transaction and anything necessary according to its status and data.


## Contributing

You can make pull requests or create issues in GitHub. Any code you send should be formatted using `make gofmt`.


## License

[MIT License](LICENSE)
