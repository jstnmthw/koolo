package paladin

import (
	"log/slog"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data/skill"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/game"
)

type PoisonCleanse struct {
	Data   *game.Data
	Logger *slog.Logger
}

func NewPoisonCleanse(data *game.Data, logger *slog.Logger) *PoisonCleanse {
	return &PoisonCleanse{
		Data:   data,
		Logger: logger,
	}
}

func (b *PoisonCleanse) IsPoisoned() bool {
	isPoisoned := b.Data.PlayerUnit.Stats[stat.PoisonLength].Value > 0
	if isPoisoned {
		b.Logger.Debug("Player is poisoned")
	} else {
		b.Logger.Debug("Player is not poisoned")
	}
	return isPoisoned
}

func (b *PoisonCleanse) CleansePoison(duration time.Duration) {
	if err := step.SetSkill(skill.Cleansing); err != nil {
		b.Logger.Warn("Failed switching to Cleansing aura")
		return
	}

	start := time.Now()
	for {
		if time.Since(start) >= duration || !b.IsPoisoned() {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
}
