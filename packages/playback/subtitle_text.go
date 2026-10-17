package playback

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	xunicode "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/language"
	"golang.org/x/text/transform"
)

var errSubtitleText = errors.New("subtitle text encoding is invalid")

func DecodeSubtitleText(data []byte, languageCode string) ([]byte, error) {
	switch {
	case bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}):
		data = data[3:]
	case bytes.HasPrefix(data, []byte{0xff, 0xfe}):
		return decodedSubtitleText(data, xunicode.UTF16(xunicode.LittleEndian, xunicode.ExpectBOM).NewDecoder())
	case bytes.HasPrefix(data, []byte{0xfe, 0xff}):
		return decodedSubtitleText(data, xunicode.UTF16(xunicode.BigEndian, xunicode.ExpectBOM).NewDecoder())
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, errSubtitleText
	}
	if utf8.Valid(data) {
		return data, nil
	}
	return decodedSubtitleText(data, legacySubtitleEncoding(languageCode).NewDecoder())
}

func SubtitleFileLanguage(path string) string {
	parts := strings.Split(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), ".")
	for index := len(parts) - 1; index > 0 && index >= len(parts)-3; index-- {
		if base, err := language.ParseBase(strings.ToLower(parts[index])); err == nil && len(parts[index]) <= 3 {
			return base.String()
		}
	}
	return ""
}

func decodedSubtitleText(data []byte, decoder *encoding.Decoder) ([]byte, error) {
	decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(data), decoder))
	if err != nil || !utf8.Valid(decoded) || bytes.IndexByte(decoded, 0) >= 0 {
		return nil, errSubtitleText
	}
	return bytes.TrimPrefix(decoded, []byte{0xef, 0xbb, 0xbf}), nil
}

func legacySubtitleEncoding(languageCode string) encoding.Encoding {
	tag, _ := language.Parse(languageCode)
	base, _ := tag.Base()
	if script, _ := tag.Script(); base.String() == "zh" && (script.String() == "Hant" || tag == language.MustParse("zh-TW") || tag == language.MustParse("zh-HK")) {
		return traditionalchinese.Big5
	}
	switch base.String() {
	case "ru", "uk", "bg", "be", "mk", "sr":
		return charmap.Windows1251
	case "pl", "cs", "sk", "hu", "hr", "sl", "ro":
		return charmap.Windows1250
	case "el":
		return charmap.Windows1253
	case "tr":
		return charmap.Windows1254
	case "he":
		return charmap.Windows1255
	case "ar", "fa", "ur":
		return charmap.Windows1256
	case "th":
		return charmap.Windows874
	case "ja":
		return japanese.ShiftJIS
	case "zh":
		return simplifiedchinese.GB18030
	case "ko":
		return korean.EUCKR
	default:
		return charmap.Windows1252
	}
}
