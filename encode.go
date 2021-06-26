package gosoap

import (
	"encoding/xml"
	"fmt"
	"reflect"
	"strconv"
)

var (
	soapPrefix                            = "soap"
	customEnvelopeAttrs map[string]string = nil
)

// SetCustomEnvelope define customizated envelope
func SetCustomEnvelope(prefix string, attrs map[string]string) {
	soapPrefix = prefix
	if attrs != nil {
		customEnvelopeAttrs = attrs
	}
}

// MarshalXML envelope the body and encode to xml
func (p process) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	tokens := &tokenData{}

	//start envelope
	if p.Client.Definitions == nil {
		return fmt.Errorf("definitions is nil")
	}

	namespace := ""
	if p.Client.Definitions.Types != nil {
		namespace = p.Client.Definitions.Types[0].XsdSchema[0].TargetNamespace
	}

	tokens.startEnvelope()
	if len(p.Client.HeaderParams) > 0 {
		tokens.startHeader(p.Client.HeaderName, namespace)

		tokens.recursiveEncode(p.Client.HeaderParams)

		tokens.endHeader(p.Client.HeaderName)
	}

	err := tokens.startBody(p.Request.Method, namespace, p.Request.Namespace)
	if err != nil {
		return err
	}

	tokens.recursiveEncode(p.Request.Params)

	//end envelope
	tokens.endBody(p.Request.Method, p.Request.Namespace)
	tokens.endEnvelope()

	for _, t := range tokens.data {
		err := e.EncodeToken(t)
		if err != nil {
			return err
		}
	}

	return e.Flush()
}

type tokenData struct {
	data []xml.Token
}

func (tokens *tokenData) recursiveEncode(hm interface{}) {
	v := reflect.ValueOf(hm)
	switch v.Kind() {
	case reflect.Struct:
		if np, ok := hm.(NamespaceParam); ok {
			t := xml.StartElement{
				Name: xml.Name{
					Space: "",
					Local: fmt.Sprintf("%s:%s", np.Namespace, np.Name),
				},
			}
			tokens.data = append(tokens.data, t)
			tokens.recursiveEncode(np.Value)
			tokens.data = append(tokens.data, xml.EndElement{Name: t.Name})
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			t := xml.StartElement{
				Name: xml.Name{
					Space: "",
					Local: key.String(),
				},
			}

			tokens.data = append(tokens.data, t)
			tokens.recursiveEncode(v.MapIndex(key).Interface())
			tokens.data = append(tokens.data, xml.EndElement{Name: t.Name})
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			tokens.recursiveEncode(v.Index(i).Interface())
		}
	case reflect.Array:
		if v.Len() == 2 {
			label := v.Index(0).Interface()
			t := xml.StartElement{
				Name: xml.Name{
					Space: "",
					Local: label.(string),
				},
			}

			tokens.data = append(tokens.data, t)
			tokens.recursiveEncode(v.Index(1).Interface())
			tokens.data = append(tokens.data, xml.EndElement{Name: t.Name})
		}
	case reflect.String:
		content := xml.CharData(v.String())
		tokens.data = append(tokens.data, content)
	case reflect.Int:
		content := xml.CharData(strconv.Itoa(int(v.Int())))
		tokens.data = append(tokens.data, content)
	}
}

func (tokens *tokenData) startEnvelope() {
	e := xml.StartElement{
		Name: xml.Name{
			Space: "",
			Local: fmt.Sprintf("%s:Envelope", soapPrefix),
		},
	}

	if customEnvelopeAttrs == nil {
		e.Attr = []xml.Attr{
			{Name: xml.Name{Space: "", Local: "xmlns:xsi"}, Value: "http://www.w3.org/2001/XMLSchema-instance"},
			{Name: xml.Name{Space: "", Local: "xmlns:xsd"}, Value: "http://www.w3.org/2001/XMLSchema"},
			{Name: xml.Name{Space: "", Local: "xmlns:soap"}, Value: "http://schemas.xmlsoap.org/soap/envelope/"},
		}
	} else {
		e.Attr = make([]xml.Attr, 0)
		for local, value := range customEnvelopeAttrs {
			e.Attr = append(e.Attr, xml.Attr{
				Name:  xml.Name{Space: "", Local: local},
				Value: value,
			})
		}
	}

	tokens.data = append(tokens.data, e)
}

func (tokens *tokenData) endEnvelope() {
	e := xml.EndElement{
		Name: xml.Name{
			Space: "",
			Local: fmt.Sprintf("%s:Envelope", soapPrefix),
		},
	}

	tokens.data = append(tokens.data, e)
}

func (tokens *tokenData) startHeader(m, n string) {
	h := xml.StartElement{
		Name: xml.Name{
			Space: "",
			Local: fmt.Sprintf("%s:Header", soapPrefix),
		},
	}

	if m == "" || n == "" {
		tokens.data = append(tokens.data, h)
		return
	}

	r := xml.StartElement{
		Name: xml.Name{
			Space: "",
			Local: m,
		},
		Attr: []xml.Attr{
			{Name: xml.Name{Space: "", Local: "xmlns"}, Value: n},
		},
	}

	tokens.data = append(tokens.data, h, r)

	return
}

func (tokens *tokenData) endHeader(m string) {
	h := xml.EndElement{
		Name: xml.Name{
			Space: "",
			Local: fmt.Sprintf("%s:Header", soapPrefix),
		},
	}

	if m == "" {
		tokens.data = append(tokens.data, h)
		return
	}

	r := xml.EndElement{
		Name: xml.Name{
			Space: "",
			Local: m,
		},
	}

	tokens.data = append(tokens.data, r, h)
}

func (tokens *tokenData) startBody(m, n, pn string) error {
	b := xml.StartElement{
		Name: xml.Name{
			Space: "",
			Local: fmt.Sprintf("%s:Body", soapPrefix),
		},
	}

	if m == "" || n == "" {
		return fmt.Errorf("method or namespace is empty")
	}

	var r xml.StartElement

	if pn != "" {
		r = xml.StartElement{
			Name: xml.Name{
				Space: "",
				Local: fmt.Sprintf("%s:%s", pn, m),
			},
		}
	} else {
		r = xml.StartElement{
			Name: xml.Name{
				Space: "",
				Local: m,
			},
			Attr: []xml.Attr{
				{Name: xml.Name{Space: "", Local: "xmlns"}, Value: n},
			},
		}
	}

	tokens.data = append(tokens.data, b, r)

	return nil
}

// endToken close body of the envelope
func (tokens *tokenData) endBody(m, np string) {
	b := xml.EndElement{
		Name: xml.Name{
			Space: "",
			Local: fmt.Sprintf("%s:Body", soapPrefix),
		},
	}

	local := m

	if np != "" {
		local = fmt.Sprintf("%s:%s", np, m)
	}

	r := xml.EndElement{
		Name: xml.Name{
			Space: "",
			Local: local,
		},
	}

	tokens.data = append(tokens.data, r, b)
}
