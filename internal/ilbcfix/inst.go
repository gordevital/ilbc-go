package ilbcfix

// Instance structs mirror the WebRTC IlbcEncoder_ / IlbcDecoder_ types
// (defines.h) exactly, using fixed-size arrays for the C state buffers.

// EncInst mirrors WebRTC IlbcEncoder_ (defines.h).
type EncInst struct {
	Mode          int
	BlockL        int
	NSub          int
	NASub         int
	NoOfBytes     int
	NoOfWords     int
	LpcN          int
	StateShortLen int

	AnaMem    [LPCFilterOrder]int16
	LsfOld    [LPCFilterOrder]int16
	LsfDeqOld [LPCFilterOrder]int16
	LpcBuffer [LPCLookBack + BlockLMax]int16
	HpInX     [2]int16
	HpInY     [4]int16
}

// DecInst mirrors WebRTC IlbcDecoder_ (defines.h).
type DecInst struct {
	Mode          int
	BlockL        int
	NSub          int
	NASub         int
	NoOfBytes     int
	NoOfWords     int
	LpcN          int
	StateShortLen int

	SyntMem      [LPCFilterOrder]int16
	LsfDeqOld    [LPCFilterOrder]int16
	LastLag      int
	ConsPLICount int
	PrevEnhPL    int
	PerSquare    int16
	PrevScale    int16
	PrevPLI      int16
	PrevLag      int
	PrevLpc      [LPCFilterOrder + 1]int16
	PrevResidual [NSubMax * SubL]int16
	Seed         int16
	OldSyntDenum [(LPCFilterOrder + 1) * NSubMax]int16
	HpInX        [2]int16
	HpInY        [4]int16
	UseEnhancer  int
	EnhBuf       [EnhBufl + EnhBuflFilterOVRhead]int16
	EnhPeriod    [EnhNBlocksTot]int
}

// iLBC_bits mirrors the WebRTC iLBC_bits struct (defines.h).
type iLBCBits struct {
	Lsf        [LSFNSplit * LpcNMax]int16
	CbIndex    [CBNStages * (NaSubMax + 1)]int16
	GainIndex  [CBNStages * (NaSubMax + 1)]int16
	IdxForMax  int
	StateFirst int16
	IdxVec     [StateShortLen30ms]int16
	FirstBits  int16
	StartIdx   int
}

// initEncode mirrors WebRtcIlbcfix_InitEncode.
func initEncode(enc *EncInst, mode int) int {
	enc.Mode = mode
	switch mode {
	case 30:
		enc.BlockL, enc.NSub, enc.NASub, enc.LpcN = BlockL30ms, NSub30ms, NaSub30ms, LpcN30ms
		enc.NoOfBytes, enc.NoOfWords, enc.StateShortLen = NoOfBytes30ms, NoOfWords30ms, StateShortLen30ms
	case 20:
		enc.BlockL, enc.NSub, enc.NASub, enc.LpcN = BlockL20ms, NSub20ms, NaSub20ms, LpcN20ms
		enc.NoOfBytes, enc.NoOfWords, enc.StateShortLen = NoOfBytes20ms, NoOfWords20ms, StateShortLen20ms
	default:
		return -1
	}

	memSetW16(enc.AnaMem[:], 0, LPCFilterOrder, 0)
	copy(enc.LsfOld[:], kLsfMean[:])
	copy(enc.LsfDeqOld[:], kLsfMean[:])
	memSetW16(enc.LpcBuffer[:], 0, LPCLookBack+BlockLMax, 0)
	memSetW16(enc.HpInX[:], 0, 2, 0)
	memSetW16(enc.HpInY[:], 0, 4, 0)

	return enc.NoOfBytes
}

// initDecode mirrors WebRtcIlbcfix_InitDecode.
func initDecode(dec *DecInst, mode, useEnhancer int) int {
	dec.Mode = mode
	switch mode {
	case 30:
		dec.BlockL, dec.NSub, dec.NASub, dec.LpcN = BlockL30ms, NSub30ms, NaSub30ms, LpcN30ms
		dec.NoOfBytes, dec.NoOfWords, dec.StateShortLen = NoOfBytes30ms, NoOfWords30ms, StateShortLen30ms
	case 20:
		dec.BlockL, dec.NSub, dec.NASub, dec.LpcN = BlockL20ms, NSub20ms, NaSub20ms, LpcN20ms
		dec.NoOfBytes, dec.NoOfWords, dec.StateShortLen = NoOfBytes20ms, NoOfWords20ms, StateShortLen20ms
	default:
		return -1
	}

	copy(dec.LsfDeqOld[:], kLsfMean[:])
	memSetW16(dec.SyntMem[:], 0, LPCFilterOrder, 0)

	memSetW16(dec.OldSyntDenum[:], 0, (LPCFilterOrder+1)*NSubMax, 0)
	for i := 0; i < NSubMax; i++ {
		dec.OldSyntDenum[i*(LPCFilterOrder+1)] = 4096
	}

	dec.LastLag = 20
	dec.ConsPLICount = 0
	dec.PrevPLI = 0
	dec.PerSquare = 0
	dec.PrevLag = 120
	dec.PrevLpc[0] = 4096
	memSetW16(dec.PrevLpc[1:], 0, LPCFilterOrder, 0)
	memSetW16(dec.PrevResidual[:], 0, BlockLMax, 0)

	dec.Seed = 777

	memSetW16(dec.HpInX[:], 0, 2, 0)
	memSetW16(dec.HpInY[:], 0, 4, 0)

	dec.UseEnhancer = useEnhancer
	memSetW16(dec.EnhBuf[:], 0, EnhBufl+EnhBuflFilterOVRhead, 0)
	for i := 0; i < EnhNBlocksTot; i++ {
		dec.EnhPeriod[i] = 160 // Q(-4)
	}

	dec.PrevEnhPL = 0

	return dec.BlockL
}
