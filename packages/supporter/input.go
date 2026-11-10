package supporter

import (
	"errors"
	"strings"
)

var (
	ErrInvalidKey             = errors.New("invalid supporter key")
	errInvalidRecognitionName = errors.New("invalid supporter recognition name")
)

func NewActivationInput(key, recognitionName string) (ActivationInput, error) {
	key = strings.TrimSpace(key)
	if !validKey(key) {
		return ActivationInput{}, ErrInvalidKey
	}
	input := ActivationInput{Key: key}
	if recognitionName == "" {
		return input, nil
	}
	if !validRecognitionName(recognitionName, true) {
		return ActivationInput{}, errInvalidRecognitionName
	}
	input.RecognitionName = &recognitionName
	return input, nil
}

func ParseActivationJSON(data []byte) (ActivationInput, error) {
	object, err := jsonObject(data)
	if err != nil || !exactFields(object, []string{"key"}, []string{"recognitionName"}) {
		return ActivationInput{}, errInvalid
	}
	key, ok := jsonString(object["key"])
	if !ok {
		return ActivationInput{}, errInvalid
	}
	input := ActivationInput{Key: key}
	if raw, found := object["recognitionName"]; found {
		name, nameOK := jsonString(raw)
		if !nameOK || !validRecognitionName(name, true) {
			return ActivationInput{}, errInvalid
		}
		input.RecognitionName = &name
	}
	return input, nil
}
