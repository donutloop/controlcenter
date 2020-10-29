package core

import (
	"encoding/json"
	"errors"
	"gitlab.com/lightmeter/controlcenter/util/errorutil"
	"log"
	"reflect"
)

var (
	ErrInvalidContentType = errors.New("Unknown Insight Content Type")
)

type decoderHandler func([]byte) (Content, error)

type converterHandler struct {
	value   int
	decoder decoderHandler
}

var (
	converters        = map[string]converterHandler{}
	reverseConverters = map[int]string{}
)

func ContentTypeForValue(value int) (string, error) {
	c, ok := reverseConverters[value]

	if !ok {
		return "", ErrInvalidContentType
	}

	return c, nil
}

func ValueForContentType(contentType string) (int, error) {
	v, ok := converters[contentType]

	if !ok {
		return 0, ErrInvalidContentType
	}

	return v.value, nil
}

func RegisterContentType(contentType string, value int, handler decoderHandler) {
	c, ok := reverseConverters[value]

	if ok {
		log.Panicln("A content converter with value", value, "is already registred:", c, ". You must use a different and unique number!")
		return
	}

	converters[contentType] = converterHandler{
		value:   value,
		decoder: handler,
	}

	reverseConverters[value] = contentType
}

func DefaultContentTypeDecoder(content Content) func(b []byte) (Content, error) {
	reflectedValue := reflect.ValueOf(content)

	if reflectedValue.Kind() != reflect.Ptr {
		panic("content is not ptr")
	}

	handler := func(b []byte) (Content, error) {
		v := reflect.New(reflectedValue.Elem().Type()).Interface().(Content)

		err := json.Unmarshal(b, v)
		if err != nil {
			return nil, errorutil.Wrap(err)
		}

		return v, nil
	}

	return handler
}

func decodeByContentType(contentType string, content []byte) (Content, error) {
	v, ok := converters[contentType]

	if !ok {
		return nil, ErrInvalidContentType
	}

	return v.decoder(content)
}
