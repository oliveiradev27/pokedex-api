package pokeapi

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Pokemon representa o formato principal devolvido pelo endpoint pokemon da PokeAPI.
// Os campos compostos tornam explícito, para fins didáticos, o contrato JSON consumido.
type Pokemon struct {
	Abilities              []PokemonAbility       `json:"abilities"`
	BaseExperience         int                    `json:"base_experience"`
	Cries                  PokemonCries           `json:"cries"`
	Forms                  []PokemonNamedResource `json:"forms"`
	GameIndices            []PokemonGameIndex     `json:"game_indices"`
	Height                 int                    `json:"height"`
	HeldItems              []PokemonHeldItem      `json:"held_items"`
	ID                     int                    `json:"id"`
	LocationAreaEncounters string                 `json:"location_area_encounters"`
	Moves                  []PokemonMove          `json:"moves"`
	Name                   string                 `json:"name"`
	Order                  int                    `json:"order"`
	PastTypes              []PokemonPastType      `json:"past_types"`
	Species                PokemonNamedResource   `json:"species"`
	Sprites                PokemonSprites         `json:"sprites"`
	Stats                  []PokemonStat          `json:"stats"`
	Types                  []PokemonType          `json:"types"`
	Weight                 int                    `json:"weight"`
}

// Berry representa o recurso berry da PokeAPI.
type Berry struct {
	Firmness         PokemonNamedResource `json:"firmness"`
	Flavors          []BerryFlavor        `json:"flavors"`
	GrowthTime       int                  `json:"growth_time"`
	ID               int                  `json:"id"`
	Item             PokemonNamedResource `json:"item"`
	MaxHarvest       int                  `json:"max_harvest"`
	Name             string               `json:"name"`
	NaturalGiftPower int                  `json:"natural_gift_power"`
	NaturalGiftType  PokemonNamedResource `json:"natural_gift_type"`
	Size             int                  `json:"size"`
	Smoothness       int                  `json:"smoothness"`
	SoilDryness      int                  `json:"soil_dryness"`
}

// Item representa o núcleo do recurso item da PokeAPI.
type Item struct {
	Attributes        []PokemonNamedResource `json:"attributes"`
	BabyTriggerFor    json.RawMessage        `json:"baby_trigger_for"`
	Category          PokemonNamedResource   `json:"category"`
	Cost              int                    `json:"cost"`
	EffectEntries     []ItemEffectEntry      `json:"effect_entries"`
	FlavorTextEntries []ItemFlavorTextEntry  `json:"flavor_text_entries"`
	FlingEffect       PokemonNamedResource   `json:"fling_effect"`
	FlingPower        int                    `json:"fling_power"`
	GameIndices       []ItemGameIndex        `json:"game_indices"`
	HeldByPokemon     []ItemHeldByPokemon    `json:"held_by_pokemon"`
	ID                int                    `json:"id"`
	Name              string                 `json:"name"`
	Names             []ItemName             `json:"names"`
	Sprites           ItemSprites            `json:"sprites"`
}

type BerryFlavor struct {
	Flavor  PokemonNamedResource `json:"flavor"`
	Potency int                  `json:"potency"`
}
type ItemEffectEntry struct {
	Effect      string               `json:"effect"`
	Language    PokemonNamedResource `json:"language"`
	ShortEffect string               `json:"short_effect"`
}
type ItemFlavorTextEntry struct {
	Language     PokemonNamedResource `json:"language"`
	Text         string               `json:"text"`
	VersionGroup PokemonNamedResource `json:"version_group"`
}
type ItemGameIndex struct {
	GameIndex  int                  `json:"game_index"`
	Generation PokemonNamedResource `json:"generation"`
}
type ItemHeldByPokemon struct {
	Pokemon        PokemonNamedResource `json:"pokemon"`
	VersionDetails []ItemVersionDetail  `json:"version_details"`
}
type ItemVersionDetail struct {
	Rarity  int                  `json:"rarity"`
	Version PokemonNamedResource `json:"version"`
}
type ItemName struct {
	Language PokemonNamedResource `json:"language"`
	Name     string               `json:"name"`
}
type ItemSprites struct {
	Default string `json:"default"`
}

type PokemonAbility struct {
	Ability  PokemonNamedResource `json:"ability"`
	IsHidden bool                 `json:"is_hidden"`
	Slot     int                  `json:"slot"`
}
type PokemonCries struct {
	Latest string `json:"latest"`
	Legacy string `json:"legacy"`
}
type PokemonNamedResource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}
type PokemonGameIndex struct {
	GameIndex int                  `json:"game_index"`
	Version   PokemonNamedResource `json:"version"`
}
type PokemonHeldItem struct {
	Item           PokemonNamedResource     `json:"item"`
	VersionDetails []PokemonHeldItemVersion `json:"version_details"`
}
type PokemonHeldItemVersion struct {
	Rarity  int                  `json:"rarity"`
	Version PokemonNamedResource `json:"version"`
}
type PokemonMove struct {
	Move                PokemonNamedResource      `json:"move"`
	VersionGroupDetails []PokemonMoveVersionGroup `json:"version_group_details"`
}
type PokemonMoveVersionGroup struct {
	LevelLearnedAt  int                  `json:"level_learned_at"`
	MoveLearnMethod PokemonNamedResource `json:"move_learn_method"`
	VersionGroup    PokemonNamedResource `json:"version_group"`
}
type PokemonPastType struct {
	Generation PokemonNamedResource `json:"generation"`
	Types      []PokemonType        `json:"types"`
}
type PokemonSprites struct {
	BackDefault      string                     `json:"back_default"`
	BackFemale       string                     `json:"back_female"`
	BackShiny        string                     `json:"back_shiny"`
	BackShinyFemale  string                     `json:"back_shiny_female"`
	FrontDefault     string                     `json:"front_default"`
	FrontFemale      string                     `json:"front_female"`
	FrontShiny       string                     `json:"front_shiny"`
	FrontShinyFemale string                     `json:"front_shiny_female"`
	Other            map[string]json.RawMessage `json:"other"`
	Versions         map[string]json.RawMessage `json:"versions"`
}
type PokemonStat struct {
	BaseStat int                  `json:"base_stat"`
	Effort   int                  `json:"effort"`
	Stat     PokemonNamedResource `json:"stat"`
}
type PokemonType struct {
	Slot int                  `json:"slot"`
	Type PokemonNamedResource `json:"type"`
}

// ParseAndSerialize valida e rematerializa o JSON recebido em structs Go.
// Recursos pokemon individuais usam Pokemon; os demais JSONs usam uma árvore genérica.
func ParseAndSerialize(resourcePath string, body []byte) ([]byte, error) {
	if isPokemonDetail(resourcePath) {
		var resource Pokemon
		if err := json.Unmarshal(body, &resource); err != nil {
			return nil, fmt.Errorf("parse pokemon: %w", err)
		}
		encoded, err := json.Marshal(resource)
		if err != nil {
			return nil, fmt.Errorf("serialize pokemon: %w", err)
		}
		return encoded, nil
	}
	var model any
	switch resourceKind(resourcePath) {
	case "berry":
		model = &Berry{}
	case "item":
		model = &Item{}
	}
	if model != nil {
		if err := json.Unmarshal(body, model); err != nil {
			return nil, fmt.Errorf("parse %s: %w", resourceKind(resourcePath), err)
		}
		encoded, err := json.Marshal(model)
		if err != nil {
			return nil, fmt.Errorf("serialize %s: %w", resourceKind(resourcePath), err)
		}
		return encoded, nil
	}
	var resource any
	if err := json.Unmarshal(body, &resource); err != nil {
		return nil, fmt.Errorf("parse recurso: %w", err)
	}
	encoded, err := json.Marshal(resource)
	if err != nil {
		return nil, fmt.Errorf("serialize recurso: %w", err)
	}
	return encoded, nil
}

func isPokemonDetail(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 2 && parts[0] == "pokemon" && parts[1] != ""
}

func resourceKind(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 2 && parts[1] != "" {
		return parts[0]
	}
	return ""
}
