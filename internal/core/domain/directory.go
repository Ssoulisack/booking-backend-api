package domain

import (
	"slices"
	"strings"
)

// SportTypeOther is the type of a sport that fits none of the others
const SportTypeOther = "other"

// SportTypes is the one list customers filter branches by; a branch's sport keeps its own
// name ("Court tennis VIP", "ເທັນນິດ") and picks one of these types
var SportTypes = []string{
	"football", "futsal", "badminton", "tennis", "table_tennis", "basketball", "volleyball",
	"boxing", "muay_thai", "swimming", "golf", "fitness", "petanque", SportTypeOther,
}

func ValidSportType(t string) bool { return slices.Contains(SportTypes, t) }

// sportWords: the first type whose word appears in a sport's name (English, Lao or Thai). More
// specific types come first: "table tennis" before "tennis", "futsal" before "football".
var sportWords = []struct {
	sportType string
	words     []string
}{
	{"table_tennis", []string{"table tennis", "ping pong", "pingpong", "ປິງປອງ", "ปิงปอง"}},
	{"futsal", []string{"futsal", "ຟຸດຊໍ", "ຟຸດຊອລ", "ฟุตซอล"}},
	{"muay_thai", []string{"muay", "kickbox", "ມວຍໄທ", "ມວຍລາວ", "มวยไทย"}},
	{"football", []string{"football", "soccer", "ບານເຕະ", "ฟุตบอล"}},
	{"badminton", []string{"badminton", "ແບດມິນຕັນ", "ຕີດອກປີກໄກ່", "แบดมินตัน"}},
	{"tennis", []string{"tennis", "ເທັນນິດ", "ເທນນິສ", "เทนนิส"}},
	{"basketball", []string{"basketball", "ບານບ້ວງ", "บาสเกตบอล", "บาส"}},
	{"volleyball", []string{"volleyball", "ບານສົ່ງ", "วอลเลย์บอล"}},
	{"boxing", []string{"boxing", "box", "ມວຍ", "มวย"}},
	{"swimming", []string{"swim", "pool", "ລອຍນ້ຳ", "ສະລອຍນ້ຳ", "ว่ายน้ำ"}},
	{"golf", []string{"golf", "ກອຟ", "กอล์ฟ"}},
	{"fitness", []string{"fitness", "gym", "yoga", "ຟິດເນດ", "ຍິມ", "ฟิตเนส", "ยิม"}},
	{"petanque", []string{"petanque", "pétanque", "boules", "ເປຕັງ", "เปตอง"}},
}

// GuessSportType gives a sport a type from its name; the owner can change it in Setup
func GuessSportType(name string) string {
	n := strings.ToLower(name)
	for _, sw := range sportWords {
		for _, w := range sw.words {
			if strings.Contains(n, w) {
				return sw.sportType
			}
		}
	}
	return SportTypeOther
}

// Provinces of Laos a branch can be in (Settings); customers filter places by them
var Provinces = []string{
	"vientiane_capital", "phongsaly", "luang_namtha", "oudomxay", "bokeo", "luang_prabang",
	"houaphanh", "xayabury", "xiengkhouang", "vientiane", "bolikhamxay", "khammouane",
	"savannakhet", "saravane", "sekong", "champasack", "attapeu", "xaisomboun",
}

// ValidProvince accepts one of Provinces, or "" (not chosen yet)
func ValidProvince(p string) bool { return p == "" || slices.Contains(Provinces, p) }
