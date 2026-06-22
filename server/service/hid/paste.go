package hid

import (
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"NanoKVM-Server/proto"
)

type Char struct {
	Modifiers int
	Code      int
}

type PasteReq struct {
	Content string `form:"content" validate:"required"`
}

func (s *Service) Paste(c *gin.Context) {
	var req PasteReq
	var rsp proto.Response

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	if len(req.Content) > 1024 {
		rsp.ErrRsp(c, -2, "content too long")
		return
	}

	keyUp := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}

	for _, char := range req.Content {
		// Accented characters (and standalone dead-key glyphs) expand to a
		// sequence of keystrokes — a dead key followed by the base letter or a
		// space. Plain characters resolve to a single keystroke via CharMap.
		seq, ok := ComposedMap[char]
		if !ok {
			key, single := CharMap[char]
			if !single {
				log.Debugf("unknown key '%c' (rune: %d)", char, char)
				continue
			}
			seq = []Char{key}
		}

		for _, key := range seq {
			keyDown := []byte{byte(key.Modifiers), 0x00, byte(key.Code), 0x00, 0x00, 0x00, 0x00, 0x00}

			hid.WriteHid0(keyDown)
			hid.WriteHid0(keyUp)
			time.Sleep(50 * time.Millisecond)
		}
	}

	rsp.OkRsp(c)
	log.Debugf("hid paste success, total %d characters processed", len(req.Content))
}

// Modifier bit positions (USB HID boot keyboard report, byte 0).
const (
	ModNone  = 0
	ModShift = 2    // Left Shift
	ModAltGr = 0x40 // Right Alt (AltGr). KBDBR treats Right Alt as Ctrl+Alt.
)

// Dead keys on the Brazilian ABNT (KBDBR) layout. They produce no character on
// their own; Windows composes them with the following base letter, or emits a
// spacing glyph when followed by Space.
//
//	acute/grave    -> OEM_4, HID 0x2F (US '[' position)
//	tilde/circumflex -> OEM_7, HID 0x34 (US '\'' position)
//	diaeresis      -> Shift + '6', HID 0x23
var (
	deadAcute      = Char{ModNone, 0x2f}  // ´
	deadGrave      = Char{ModShift, 0x2f} // `
	deadTilde      = Char{ModNone, 0x34}  // ~
	deadCircumflex = Char{ModShift, 0x34} // ^
	deadDiaeresis  = Char{ModShift, 0x23} // ¨
	keySpace       = Char{ModNone, 0x2c}
)

// CharMap maps a rune to the single HID keystroke that produces it on a target
// running the Brazilian Portuguese ABNT / ABNT2 layout (Windows KBDBR.DLL).
// HID codes are physical key positions, so they differ from a US keymap for
// every symbol whose position moved between layouts.
var CharMap = map[rune]Char{
	// Lowercase letters (same positions as US QWERTY)
	'a': {ModNone, 0x04}, 'b': {ModNone, 0x05}, 'c': {ModNone, 0x06}, 'd': {ModNone, 0x07},
	'e': {ModNone, 0x08}, 'f': {ModNone, 0x09}, 'g': {ModNone, 0x0a}, 'h': {ModNone, 0x0b},
	'i': {ModNone, 0x0c}, 'j': {ModNone, 0x0d}, 'k': {ModNone, 0x0e}, 'l': {ModNone, 0x0f},
	'm': {ModNone, 0x10}, 'n': {ModNone, 0x11}, 'o': {ModNone, 0x12}, 'p': {ModNone, 0x13},
	'q': {ModNone, 0x14}, 'r': {ModNone, 0x15}, 's': {ModNone, 0x16}, 't': {ModNone, 0x17},
	'u': {ModNone, 0x18}, 'v': {ModNone, 0x19}, 'w': {ModNone, 0x1a}, 'x': {ModNone, 0x1b},
	'y': {ModNone, 0x1c}, 'z': {ModNone, 0x1d},

	// Uppercase letters (Shift + base)
	'A': {ModShift, 0x04}, 'B': {ModShift, 0x05}, 'C': {ModShift, 0x06}, 'D': {ModShift, 0x07},
	'E': {ModShift, 0x08}, 'F': {ModShift, 0x09}, 'G': {ModShift, 0x0a}, 'H': {ModShift, 0x0b},
	'I': {ModShift, 0x0c}, 'J': {ModShift, 0x0d}, 'K': {ModShift, 0x0e}, 'L': {ModShift, 0x0f},
	'M': {ModShift, 0x10}, 'N': {ModShift, 0x11}, 'O': {ModShift, 0x12}, 'P': {ModShift, 0x13},
	'Q': {ModShift, 0x14}, 'R': {ModShift, 0x15}, 'S': {ModShift, 0x16}, 'T': {ModShift, 0x17},
	'U': {ModShift, 0x18}, 'V': {ModShift, 0x19}, 'W': {ModShift, 0x1a}, 'X': {ModShift, 0x1b},
	'Y': {ModShift, 0x1c}, 'Z': {ModShift, 0x1d},

	// Cedilla — dedicated ABNT key (OEM_1, HID 0x33, US ';' position)
	'ç': {ModNone, 0x33},
	'Ç': {ModShift, 0x33},

	// Numbers
	'1': {ModNone, 0x1e}, '2': {ModNone, 0x1f}, '3': {ModNone, 0x20}, '4': {ModNone, 0x21},
	'5': {ModNone, 0x22}, '6': {ModNone, 0x23}, '7': {ModNone, 0x24}, '8': {ModNone, 0x25},
	'9': {ModNone, 0x26}, '0': {ModNone, 0x27},

	// Shifted number row (ABNT: matches US except '6', which is the diaeresis dead key)
	'!': {ModShift, 0x1e}, // Shift + 1
	'@': {ModShift, 0x1f}, // Shift + 2
	'#': {ModShift, 0x20}, // Shift + 3
	'$': {ModShift, 0x21}, // Shift + 4
	'%': {ModShift, 0x22}, // Shift + 5
	'&': {ModShift, 0x24}, // Shift + 7
	'*': {ModShift, 0x25}, // Shift + 8
	'(': {ModShift, 0x26}, // Shift + 9
	')': {ModShift, 0x27}, // Shift + 0

	// Minus / equals (OEM_MINUS 0x2D, OEM_PLUS 0x2E)
	'-': {ModNone, 0x2d}, '_': {ModShift, 0x2d},
	'=': {ModNone, 0x2e}, '+': {ModShift, 0x2e},

	// Quotes — OEM_3 (HID 0x35, US backtick position) on ABNT
	'\'': {ModNone, 0x35},  // apostrophe
	'"':  {ModShift, 0x35}, // double quote

	// Brackets / braces — OEM_6 (0x30) and OEM_5 (0x31)
	'[': {ModNone, 0x30}, '{': {ModShift, 0x30},
	']': {ModNone, 0x31}, '}': {ModShift, 0x31},

	// Backslash / pipe — OEM_102 (HID 0x64, ISO key left of Z)
	'\\': {ModNone, 0x64},
	'|':  {ModShift, 0x64},

	// Comma / period (OEM_COMMA 0x36, OEM_PERIOD 0x37)
	',': {ModNone, 0x36}, '<': {ModShift, 0x36},
	'.': {ModNone, 0x37}, '>': {ModShift, 0x37},

	// Semicolon / colon — OEM_2 (HID 0x38, US '/' position) on ABNT
	';': {ModNone, 0x38},
	':': {ModShift, 0x38},

	// Slash / question — AltGr layer of Q / W (keycodes 0x14 / 0x1a).
	// The dedicated ABNT key for these is International1 (HID 0x87), but some
	// USB-gadget keyboard report descriptors cap the key-array Usage Maximum at
	// 0x65, which would silently drop 0x87. AltGr+Q / AltGr+W produce the same
	// glyphs on KBDBR while staying inside the always-supported keycode range.
	'/': {ModAltGr, 0x14}, // AltGr + Q
	'?': {ModAltGr, 0x1a}, // AltGr + W

	// Ordinal indicators — AltGr layer
	'ª': {ModAltGr, 0x30},
	'º': {ModAltGr, 0x31},

	// Whitespace
	' ':  {ModNone, 0x2c}, // Space
	'\n': {ModNone, 0x28}, // Enter
	'\t': {ModNone, 0x2b}, // Tab
}

// ComposedMap maps an accented rune (or a standalone dead-key glyph) to the
// keystroke sequence that produces it on the ABNT layout: a dead key followed
// by the base letter, or by Space for the spacing glyph.
var ComposedMap = map[rune][]Char{
	// Acute
	'á': {deadAcute, {ModNone, 0x04}},
	'é': {deadAcute, {ModNone, 0x08}},
	'í': {deadAcute, {ModNone, 0x0c}},
	'ó': {deadAcute, {ModNone, 0x12}},
	'ú': {deadAcute, {ModNone, 0x18}},
	'Á': {deadAcute, {ModShift, 0x04}},
	'É': {deadAcute, {ModShift, 0x08}},
	'Í': {deadAcute, {ModShift, 0x0c}},
	'Ó': {deadAcute, {ModShift, 0x12}},
	'Ú': {deadAcute, {ModShift, 0x18}},

	// Tilde
	'ã': {deadTilde, {ModNone, 0x04}},
	'õ': {deadTilde, {ModNone, 0x12}},
	'ñ': {deadTilde, {ModNone, 0x11}},
	'Ã': {deadTilde, {ModShift, 0x04}},
	'Õ': {deadTilde, {ModShift, 0x12}},
	'Ñ': {deadTilde, {ModShift, 0x11}},

	// Circumflex
	'â': {deadCircumflex, {ModNone, 0x04}},
	'ê': {deadCircumflex, {ModNone, 0x08}},
	'î': {deadCircumflex, {ModNone, 0x0c}},
	'ô': {deadCircumflex, {ModNone, 0x12}},
	'û': {deadCircumflex, {ModNone, 0x18}},
	'Â': {deadCircumflex, {ModShift, 0x04}},
	'Ê': {deadCircumflex, {ModShift, 0x08}},
	'Ô': {deadCircumflex, {ModShift, 0x12}},

	// Grave
	'à': {deadGrave, {ModNone, 0x04}},
	'è': {deadGrave, {ModNone, 0x08}},
	'ò': {deadGrave, {ModNone, 0x12}},
	'À': {deadGrave, {ModShift, 0x04}},

	// Diaeresis
	'ü': {deadDiaeresis, {ModNone, 0x18}},
	'Ü': {deadDiaeresis, {ModShift, 0x18}},

	// Standalone dead-key glyphs (dead key + Space)
	'´': {deadAcute, keySpace},
	'`': {deadGrave, keySpace},
	'~': {deadTilde, keySpace},
	'^': {deadCircumflex, keySpace},
	'¨': {deadDiaeresis, keySpace},
}
