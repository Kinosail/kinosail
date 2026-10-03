package playback

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func TestHLSRecipeEnumCacheIdentity(t *testing.T) {
	t.Parallel()
	policy := HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}
	for _, codec := range transcodepolicy.Codecs() {
		if !codec.Supported {
			continue
		}
		for _, burn := range []string{"none", "text", "image", "external"} {
			token := "t-a1-s7-" + burn + "-t1-b4000000-c" + codec.ID + "-o12300"
			recipe, err := ParseHLSRecipe(token, policy)
			if err != nil || recipe.Codec != codec.ID || recipe.Offset != 12.3 {
				t.Fatalf("catalog recipe %q = %#v, %v", token, recipe, err)
			}
			wantBurn := burn
			if wantBurn == "none" {
				wantBurn = ""
			}
			if recipe.Burn != wantBurn {
				t.Fatalf("burn mode %q = %q", burn, recipe.Burn)
			}
			canonical := token
			if codec.ID == "h264" {
				canonical = strings.Replace(token, "-ch264", "", 1)
			}
			if recipe.Token() != canonical || HLSRecipeKey("0123456789abcdef", recipe) != "0123456789abcdef-plan-"+canonical {
				t.Fatalf("cache identity changed for %q", token)
			}
		}
	}
}

func TestHLSRecipeRejectsUntrustedEnums(t *testing.T) {
	t.Parallel()
	policy := HLSRecipePolicy{MaxBitrate: 100_000_000, OffsetStepMilliseconds: 100}
	for _, value := range []string{"", "auto", "vvc", "av2", "unknown", "HEVC", "hevc/../", `hevc\..\`, "hevc%2f..", "hevc\x00", "hevc\n", "ｈｅｖｃ"} {
		if _, err := ParseHLSRecipe("t-a0-s0-none-t0-b0-c"+value, policy); err == nil {
			t.Errorf("accepted codec %q", value)
		}
	}
	for _, value := range []string{"", "NONE", "unknown", "external/../", `external\..\`, "external%2f..", "external\x00", "external\n", "ｅｘｔｅｒｎａｌ"} {
		if _, err := ParseHLSRecipe("t-a0-s0-"+value+"-t0-b0", policy); err == nil {
			t.Errorf("accepted burn mode %q", value)
		}
	}
}
