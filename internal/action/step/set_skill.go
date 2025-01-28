package step

import (
	"fmt"

	"github.com/hectorgimenez/d2go/pkg/data/skill"
	"github.com/hectorgimenez/koolo/internal/context"
)

func SetSkill(id skill.ID, throw ...bool) error {
	ctx := context.Get()
	ctx.SetLastStep("SetSkill")

	kb, found := ctx.Data.KeyBindings.KeyBindingForSkill(id)
	if !found && (len(throw) == 0 || !throw[0]) {
		return fmt.Errorf("keybinding for skill %s not found", id.Desc().Name)
	}

	if ctx.Data.PlayerUnit.RightSkill != id {
		ctx.HID.PressKeyBinding(kb)
	}

	return nil
}
