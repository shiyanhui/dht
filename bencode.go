package dht

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// find returns the index of first target in data starting from `start`.
// It returns -1 if target not found.
func find(data []byte, start int, target rune) (index int) {
	index = bytes.IndexRune(data[start:], target)
	if index != -1 {
		return index + start
	}
	return index
}

// DecodeString decodes a string in the data. It returns a tuple
// (decoded result, the end position, error).
func DecodeString(data []byte, start int) (
	result interface{}, index int, err error) {

	if start >= len(data) || data[start] < '0' || data[start] > '9' {
		err = errors.New("invalid string bencode")
		return
	}

	i := find(data, start, ':')
	if i == -1 {
		err = errors.New("':' not found when decode string")
		return
	}

	length, err := strconv.Atoi(string(data[start:i]))
	if err != nil {
		return
	}

	if length < 0 {
		err = errors.New("invalid length of string")
		return
	}

	index = i + 1 + length

	if index > len(data) || index < i+1 {
		err = errors.New("out of range")
		return
	}

	result = string(data[i+1 : index])
	return
}

// DecodeInt decodes int value in the data.
func DecodeInt(data []byte, start int) (
	result interface{}, index int, err error) {

	if start >= len(data) || data[start] != 'i' {
		err = errors.New("invalid int bencode")
		return
	}

	index = find(data, start+1, 'e')

	if index == -1 {
		err = errors.New("':' not found when decode int")
		return
	}

	result, err = strconv.Atoi(string(data[start+1 : index]))
	if err != nil {
		return
	}
	index++

	return
}

// decodeItem decodes an item of dict or list.
func decodeItem(data []byte, i int) (
	result interface{}, index int, err error) {

	var decodeFunc = []func([]byte, int) (interface{}, int, error){
		DecodeString, DecodeInt, DecodeList, DecodeDict,
	}

	for _, f := range decodeFunc {
		result, index, err = f(data, i)
		if err == nil {
			return
		}
	}

	err = errors.New("invalid bencode when decode item")
	return
}

// DecodeList decodes a list value.
func DecodeList(data []byte, start int) (
	result interface{}, index int, err error) {

	if start >= len(data) || data[start] != 'l' {
		err = errors.New("invalid list bencode")
		return
	}

	var item interface{}
	r := make([]interface{}, 0, 8)

	index = start + 1
	for index < len(data) {
		char, _ := utf8.DecodeRune(data[index:])
		if char == 'e' {
			break
		}

		item, index, err = decodeItem(data, index)
		if err != nil {
			return
		}
		r = append(r, item)
	}

	if index == len(data) {
		err = errors.New("'e' not found when decode list")
		return
	}
	index++

	result = r
	return
}

// DecodeDict decodes a map value.
func DecodeDict(data []byte, start int) (
	result interface{}, index int, err error) {

	if start >= len(data) || data[start] != 'd' {
		err = errors.New("invalid dict bencode")
		return
	}

	var item, key interface{}
	r := make(map[string]interface{})

	index = start + 1
	for index < len(data) {
		char, _ := utf8.DecodeRune(data[index:])
		if char == 'e' {
			break
		}

		if !unicode.IsDigit(char) {
			err = errors.New("invalid dict bencode")
			return
		}

		key, index, err = DecodeString(data, index)
		if err != nil {
			return
		}

		if index >= len(data) {
			err = errors.New("out of range")
			return
		}

		item, index, err = decodeItem(data, index)
		if err != nil {
			return
		}

		r[key.(string)] = item
	}

	if index == len(data) {
		err = errors.New("'e' not found when decode dict")
		return
	}
	index++

	result = r
	return
}

// Decode decodes a bencoded string to string, int, list or map.
func Decode(data []byte) (result interface{}, err error) {
	result, _, err = decodeItem(data, 0)
	return
}

// EncodeString encodes a string value.
func EncodeString(data string) string {
	return strings.Join([]string{strconv.Itoa(len(data)), data}, ":")
}

// EncodeInt encodes a int value.
func EncodeInt(data int) string {
	return strings.Join([]string{"i", strconv.Itoa(data), "e"}, "")
}

// EncodeItem encodes an item of dict or list.
func encodeItem(data interface{}) (item string) {
	switch v := data.(type) {
	case string:
		item = EncodeString(v)
	case int:
		item = EncodeInt(v)
	case []interface{}:
		item = EncodeList(v)
	case map[string]interface{}:
		item = EncodeDict(v)
	default:
		panic("invalid type when encode item")
	}
	return
}

// EncodeList encodes a list value.
func EncodeList(data []interface{}) string {
	result := make([]string, len(data))

	for i, item := range data {
		result[i] = encodeItem(item)
	}

	return strings.Join([]string{"l", strings.Join(result, ""), "e"}, "")
}

// EncodeDict encodes a dict value.
func EncodeDict(data map[string]interface{}) string {
	result, i := make([]string, len(data)), 0

	for key, val := range data {
		result[i] = strings.Join(
			[]string{EncodeString(key), encodeItem(val)},
			"")
		i++
	}

	return strings.Join([]string{"d", strings.Join(result, ""), "e"}, "")
}

// Encode encodes a string, int, dict or list value to a bencoded string.
func Encode(data interface{}) string {
	switch v := data.(type) {
	case string:
		return EncodeString(v)
	case int:
		return EncodeInt(v)
	case []interface{}:
		return EncodeList(v)
	case map[string]interface{}:
		return EncodeDict(v)
	default:
		panic("invalid type when encode")
	}
}
