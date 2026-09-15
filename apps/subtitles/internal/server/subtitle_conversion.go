package server

import (
	"bytes"
	"errors"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	xunicode "golang.org/x/text/encoding/unicode"
)

type subtitleConversionOptions struct {
	Encoding      string `json:"encoding"`
	RemoveCredits bool   `json:"removeCredits"`
	MergeRepeated bool   `json:"mergeRepeated"`
}

var subtitleEncodings = []string{"auto", "utf-8", "utf-16le", "utf-16be", "windows-1252", "windows-1250", "windows-1251", "windows-1253", "windows-1254", "windows-1255", "windows-1256", "windows-874", "shift-jis", "gb18030", "big5", "euc-kr"}

func convertSubtitle(data []byte, language string, options subtitleConversionOptions) (cleanedSubtitle, error) {
	if len(data) == 0 || len(data) > 4<<20 || !validLanguage(language) {
		return cleanedSubtitle{}, errors.New("subtitle conversion input is invalid")
	}
	name := options.Encoding
	if name == "" {
		name = "auto"
	}
	if !oneOf(name, subtitleEncodings...) {
		return cleanedSubtitle{}, errors.New("subtitle encoding is unsupported")
	}
	original := data
	legacy := name == "auto" && !utf8.Valid(data) && !bytes.HasPrefix(data, []byte{0xff, 0xfe}) && !bytes.HasPrefix(data, []byte{0xfe, 0xff})
	if legacy {
		name = subtitleLanguageEncoding(language)
	}
	decoded, err := decodeSubtitleEncoding(data, name)
	if err != nil {
		return cleanedSubtitle{}, err
	}
	cleaned, err := cleanSubtitle(decoded)
	if err != nil {
		return cleanedSubtitle{}, err
	}
	cleaned.Original = append([]byte(nil), original...)
	if options.RemoveCredits {
		for i := range cleaned.Cues {
			if i < 3 || i >= len(cleaned.Cues)-3 {
				cleaned.Cues[i].Text = removeSubtitleCredits(cleaned.Cues[i].Text)
			}
		}
		cleaned.Cues = slicesWithoutEmptyCues(cleaned.Cues)
		cleaned.Cleanup = append(cleaned.Cleanup, "Removed subtitle credits at the edges")
	}
	if options.MergeRepeated {
		var removed int
		cleaned.Cues, removed = mergeRepeatedSubtitleCues(cleaned.Cues)
		cleaned.Duplicates += removed
		cleaned.Cleanup = append(cleaned.Cleanup, "Merged adjacent repeated cues")
	}
	if len(cleaned.Cues) == 0 {
		return cleanedSubtitle{}, errors.New("subtitle has no dialogue cues")
	}
	document, err := subtitleDocument(cleaned.Cues, cleaned.Duplicates)
	if err != nil {
		return cleanedSubtitle{}, err
	}
	cleaned.Data, cleaned.MaxCPS = document.Data, document.MaxCPS
	if legacy {
		cleaned.Cleanup = append(cleaned.Cleanup, "Assumed "+name+" from language; check the preview")
	}
	return cleaned, nil
}

func subtitleLanguageEncoding(language string) string {
	language = strings.ToLower(canonicalSubtitleLanguage(language))
	if strings.HasPrefix(language, "zh-hant") || language == "zh-tw" || language == "zh-hk" {
		return "big5"
	}
	base := strings.Split(language, "-")[0]
	switch base {
	case "ru", "uk", "bg", "be", "mk", "sr":
		return "windows-1251"
	case "pl", "cs", "sk", "hu", "hr", "sl", "ro":
		return "windows-1250"
	case "el":
		return "windows-1253"
	case "tr":
		return "windows-1254"
	case "he":
		return "windows-1255"
	case "ar", "fa", "ur":
		return "windows-1256"
	case "th":
		return "windows-874"
	case "ja":
		return "shift-jis"
	case "zh":
		return "gb18030"
	case "ko":
		return "euc-kr"
	default:
		return "windows-1252"
	}
}

func decodeSubtitleEncoding(data []byte, name string) ([]byte, error) {
	if name == "auto" {
		return decodeSubtitleText(data)
	}
	if name == "utf-8" {
		data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
		if !utf8.Valid(data) || bytes.ContainsRune(data, 0) {
			return nil, errors.New("subtitle UTF-8 is invalid")
		}
		return data, nil
	}
	encodings := map[string]encoding.Encoding{
		"utf-16le":     xunicode.UTF16(xunicode.LittleEndian, xunicode.UseBOM),
		"utf-16be":     xunicode.UTF16(xunicode.BigEndian, xunicode.UseBOM),
		"windows-1250": charmap.Windows1250, "windows-1251": charmap.Windows1251,
		"windows-1252": charmap.Windows1252, "windows-1253": charmap.Windows1253,
		"windows-1254": charmap.Windows1254, "windows-1255": charmap.Windows1255,
		"windows-1256": charmap.Windows1256, "windows-874": charmap.Windows874,
		"shift-jis": japanese.ShiftJIS, "gb18030": simplifiedchinese.GB18030,
		"big5": traditionalchinese.Big5, "euc-kr": korean.EUCKR,
	}
	selected := encodings[name]
	if selected == nil {
		return nil, errors.New("subtitle encoding is unsupported")
	}
	return subtitleDecodedText(data, selected.NewDecoder())
}
