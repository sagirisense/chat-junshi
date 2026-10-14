package modelapi

import (
	"encoding/json"
	"testing"
)

func TestStickerOutputValidation(t *testing.T) {
	var result map[string]interface{}
	json.Unmarshal([]byte(syntheticResult), &result)
	row := result["candidates"].([]interface{})[0].(map[string]interface{})
	row["sticker_id"] = "local-id"
	row["sticker_reason"] = "轻松回应"
	row["send_order"] = "text_first"
	if err := ValidateResult(result, "reply"); err != nil {
		t.Fatal(err)
	}
	row["send_order"] = "image_only"
	if ValidateResult(result, "reply") == nil {
		t.Fatal("image only contains text")
	}
	row["text"] = ""
	if err := ValidateResult(result, "reply"); err != nil {
		t.Fatal(err)
	}
	row["sticker_id"] = ""
	if ValidateResult(result, "reply") == nil {
		t.Fatal("empty ID with image order")
	}
	row["send_order"] = "none"
	row["sticker_reason"] = ""
	if err := ValidateResult(result, "reply"); err != nil {
		t.Fatal(err)
	}
	row["sticker_reason"] = "认真解释更适合纯文字"
	if err := ValidateResult(result, "reply"); err != nil {
		t.Fatal(err)
	}
	delete(row, "sticker_reason")
	if ValidateResult(result, "reply") == nil {
		t.Fatal("accepted incomplete sticker fields")
	}
}
