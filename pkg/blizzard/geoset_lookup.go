package blizzard

import "fmt"

var geosetGroups = map[uint16]string{
	0:  "Hair",
	1:  "FacialA",
	2:  "FacialB",
	3:  "FacialC",
	4:  "Gloves",
	5:  "Boots",
	6:  "Tail",
	7:  "Ears",
	8:  "Wrists",
	9:  "Kneepads",
	10: "Chest",
	11: "Pants",
	12: "Tabard",
	13: "Trousers",
	14: "Loincloth",
	15: "Cloak",
	16: "FacialJewelry",
	17: "Eyeglow",
	18: "Belt",
	19: "Bone/Tail",
	20: "Feet",
	21: "Skull",
	22: "Torso",
	23: "HandAttach",
	24: "HeadAttach",
	25: "DHBlindfolds",
	26: "Shoulders",
	27: "Helm",
	28: "ArmUpper",
	29: "MechagnomeArms",
	30: "MechagnomeLegs",
	31: "MechagnomeFeet",
	32: "HeadSwap",
	33: "Eyes",
	34: "Eyebrows",
	35: "Piercings",
	36: "Necklace",
	37: "Headdress",
	38: "Tails",
	39: "MiscAccessory",
	40: "MiscFeature",
	41: "Noses",
	42: "HairDecoA",
	43: "HornDeco",
	44: "BodySize",
	46: "Dracthyr",
	51: "EyeGlowB",
}

func GetGeosetName(id uint16) string {
	if id == 0 {
		return "Skin"
	}

	groupId := id / 100
	subId := id - (groupId * 100)
	if groupName, ok := geosetGroups[groupId]; ok {
		return fmt.Sprintf("%s%d", groupName, subId)
	} else {
		return fmt.Sprintf("%d_%d", groupId, subId)
	}
}

func IsGeosetEquipment(id uint16) bool {
	groupId := id / 100
	subId := id - (groupId * 100)

	return ((groupId >= 4 && groupId <= 6) || (groupId >= 8 && groupId <= 13) || groupId == 15 || groupId == 18 || groupId == 20 || groupId == 22) && subId != 1
}

func IsGeosetStatic(id uint16) bool {
	groupId := id / 100
	return id == 0 || id == 401 || id == 501 || id == 1301 || id == 1801 || id == 2001 || id == 2201 || id == 2301 || id == 3301 || groupId == 32
	// so some models use the HeadSwap meshes to build different parts of the face (night elf female) and some use them for different face options (human female)
}
