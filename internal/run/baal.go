package run

import (
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/object"
	"github.com/hectorgimenez/d2go/pkg/data/skill"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/action/step/paladin"
	"github.com/hectorgimenez/koolo/internal/character"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/game"
	"github.com/hectorgimenez/koolo/internal/pather"
	"github.com/hectorgimenez/koolo/internal/utils"
)

var baalThronePosition = data.Position{
	X: 15094,
	Y: 5029,
}

type WaveMonster struct {
	ID   npc.ID
	Type data.MonsterType
}

var waveMonsters = []WaveMonster{
	{ID: npc.WarpedShaman, Type: data.MonsterTypeSuperUnique},      // Wave 1
	{ID: npc.BaalSubjectMummy, Type: data.MonsterTypeSuperUnique},  // Wave 2
	{ID: npc.CouncilMemberBall, Type: data.MonsterTypeSuperUnique}, // Wave 3
	{ID: npc.VenomLord2, Type: data.MonsterTypeSuperUnique},        // Wave 4
	{ID: npc.BaalsMinion, Type: data.MonsterTypeMinion},            // Wave 5
}

type Baal struct {
	ctx                *context.Status
	clearMonsterFilter data.MonsterFilter // Used to clear area (basically TZ)
	Logger             *slog.Logger
	poisonCleanse      *paladin.PoisonCleanse
}

func NewBaal(clearMonsterFilter data.MonsterFilter) *Baal {
	ctx := context.Get()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	return &Baal{
		ctx:                ctx,
		clearMonsterFilter: clearMonsterFilter,
		Logger:             logger,
		poisonCleanse:      paladin.NewPoisonCleanse(ctx.Data, logger),
	}
}

func (s Baal) Name() string {
	return string(config.BaalRun)
}

func (s Baal) Run() error {
	// Set filter
	filter := data.MonsterAnyFilter()
	if s.ctx.CharacterCfg.Game.Baal.OnlyElites {
		filter = data.MonsterEliteFilter()
	}
	if s.clearMonsterFilter != nil {
		filter = s.clearMonsterFilter
	}

	err := action.WayPoint(area.TheWorldStoneKeepLevel2)
	if err != nil {
		return err
	}

	if s.ctx.CharacterCfg.Game.Baal.ClearFloors || s.clearMonsterFilter != nil {
		action.ClearCurrentLevel(false, filter)
	}

	err = action.MoveToArea(area.TheWorldStoneKeepLevel3)
	if err != nil {
		return err
	}

	if s.ctx.CharacterCfg.Game.Baal.ClearFloors || s.clearMonsterFilter != nil {
		action.ClearCurrentLevel(false, filter)
	}

	err = action.MoveToArea(area.ThroneOfDestruction)
	if err != nil {
		return err
	}
	err = action.MoveToCoords(baalThronePosition)
	if err != nil {
		return err
	}
	if s.checkForSoulsOrDolls() {
		return errors.New("souls or dolls detected, skipping")
	}

	// Let's move to a safe area and open the portal in companion mode
	if s.ctx.CharacterCfg.Companion.Leader {
		action.MoveToCoords(data.Position{
			X: 15116,
			Y: 5071,
		})
		action.OpenTPIfLeader()
	}

	err = action.ClearAreaAroundPlayer(29, data.MonsterAnyFilter())
	if err != nil {
		return err
	}

	// Come back to previous position
	err = action.MoveToCoords(baalThronePosition)
	if err != nil {
		return err
	}

	// Force rebuff before waves
	action.Buff()

	// Handle Baal waves
	lastWave := false
	waveNumber := 0
	lastHandledWave := 0

	for !lastWave {
		// Check for waves based on monsters detected
		for i, monster := range waveMonsters {
			if _, found := s.ctx.Data.Monsters.FindOne(monster.ID, monster.Type); found {
				if monster.ID == npc.BaalsMinion {
					lastWave = true
					s.ctx.Logger.Debug("Last Baal wave detected.")
					continue
				}
				waveNumber = i + 1
				break
			}
		}

		// Clear current wave
		err = s.clearWave()
		if err != nil {
			return err
		}

		// Return to throne position between waves
		err = action.MoveToCoords(baalThronePosition)
		if err != nil {
			return err
		}

		// If no monsters are detected and we have a wave number,
		// handle the post-wave logic only once per waveNumber.
		if waveNumber > 0 && len(s.ctx.Data.Monsters.Enemies(data.MonsterAnyFilter())) == 0 && waveNumber != lastHandledWave {
			s.ctx.Logger.Debug("No monsters detected, post wave clear on wave", slog.Int("waveNumber", waveNumber))
			s.handlePostWaveClear(waveNumber)
			lastHandledWave = waveNumber
		}

		// Small delay to allow next wave to spawn if not last wave
		if !lastWave {
			utils.Sleep(500)
		}
	}

	_, isLevelingChar := s.ctx.Char.(context.LevelingCharacter)
	if s.ctx.CharacterCfg.Game.Baal.KillBaal || isLevelingChar {
		utils.Sleep(15000)
		action.Buff()

		// Exception: Baal portal has no destination in memory
		baalPortal, _ := s.ctx.Data.Objects.FindOne(object.BaalsPortal)
		err = action.InteractObject(baalPortal, func() bool {
			return s.ctx.Data.PlayerUnit.Area == area.TheWorldstoneChamber
		})
		if err != nil {
			return err
		}

		_ = action.MoveToCoords(data.Position{X: 15136, Y: 5943})

		return s.ctx.Char.KillBaal()
	}

	return nil
}

func (s Baal) clearWave() error {
	return s.ctx.Char.KillMonsterSequence(func(d game.Data) (data.UnitID, bool) {
		for _, m := range d.Monsters.Enemies(data.MonsterAnyFilter()) {
			dist := pather.DistanceFromPoint(baalThronePosition, m.Position)
			if d.AreaData.IsWalkable(m.Position) && dist <= 45 {
				return m.UnitID, true
			}
		}
		return 0, false
	}, nil)
}

func (s Baal) checkForSoulsOrDolls() bool {
	var npcIds []npc.ID

	if s.ctx.CharacterCfg.Game.Baal.DollQuit {
		npcIds = append(npcIds, npc.UndeadStygianDoll2, npc.UndeadSoulKiller2)
	}
	if s.ctx.CharacterCfg.Game.Baal.SoulQuit {
		npcIds = append(npcIds, npc.BlackSoul2, npc.BurningSoul2)
	}

	for _, id := range npcIds {
		if _, found := s.ctx.Data.Monsters.FindOne(id, data.MonsterTypeNone); found {
			return true
		}
	}

	return false
}

func (s Baal) handlePostWaveClear(waveNumber int) {
	switch c := s.ctx.Char.(type) {
	case character.Hammerdin:
		s.handleHammerdinAuras(waveNumber, c)
	// Extend to other character types
	default:
		return
	}
}

func (s Baal) handleHammerdinAuras(waveNumber int, char character.Hammerdin) {
	hasCleansing := char.HasSkillBound(skill.Cleansing)
	hasSalvation := char.HasSkillBound(skill.Salvation)

	if waveNumber == 0 {
		s.ctx.Logger.Debug("No wave detected, not handling auras")
		return
	}

	if waveNumber == 2 && hasCleansing {
		s.ctx.Logger.Debug("Cleansing poison for 4 seconds on wave 2")
		s.poisonCleanse.CleansePoison(6 * time.Second)
		return
	}

	if waveNumber == 3 && hasSalvation {
		s.ctx.Logger.Debug("Switching to Salvation aura on wave 3")
		if err := step.SetSkill(skill.Salvation); err != nil {
			s.ctx.Logger.Warn("Failed switching to Salvation aura")
			return
		}
		time.Sleep(5 * time.Second)
	}

	// Needs better logic not to interfere with other skills
	// if waveNumber != 2 && s.poisonCleanse.IsPoisoned() && hasCleansing {
	// 	s.ctx.Logger.Debug("Poisoned, cleansing for 2 seconds")
	// 	s.poisonCleanse.CleansePoison(2 * time.Second)
	// 	return
	// }

	return
}
