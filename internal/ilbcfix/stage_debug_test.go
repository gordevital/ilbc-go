package ilbcfix

import (
	"fmt"
	"testing"
)

func TestStageDebugSine30Frame0(t *testing.T) {
	StageDebug = true
	defer func() { StageDebug = false }()
	payload := readGoldenPayload(t, "sine30.payload")
	var dec DecInst
	InitDecoder(&dec, 30)
	var st int16
	got := make([]int16, 240)
	for slot := 0; slot < 5; slot++ {
		stagePrint(fmt.Sprintf("SLOT %d", slot))
		if slot == 3 {
			if n := DecodePlc(&dec, got, 1); n != 240 {
				t.Fatalf("slot %d: DecodePlc=%d", slot, n)
			}
		} else {
			idx := slot
			if slot > 3 {
				idx = slot - 1
			}
			if n := Decode(&dec, payload[idx*50:(idx+1)*50], got, &st); n != 240 {
				t.Fatalf("slot %d: Decode=%d", slot, n)
			}
		}
	}
}
