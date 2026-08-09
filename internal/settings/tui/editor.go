package tui

import (
	"github.com/73ai/openbotkit/settings"
	"github.com/charmbracelet/huh"
)

func buildForm(f *settings.Field, current string, svc *settings.Service) (*huh.Form, *string, *bool) {
	var strVal string
	var boolVal bool

	switch f.Type {
	case settings.TypeSelect:
		strVal = current
		options := svc.ResolvedOptions(f)
		var opts []huh.Option[string]
		for _, o := range options {
			opts = append(opts, huh.NewOption(o.Label, o.Value))
		}
		return huh.NewForm(
			huh.NewGroup(
				huh.NewSelect[string]().
					Title(f.Label).
					Description(f.Description).
					Options(opts...).
					Value(&strVal),
			),
		), &strVal, nil

	case settings.TypeBool:
		boolVal = current == "true"
		return huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(f.Label).
					Description(f.Description).
					Value(&boolVal),
			),
		), nil, &boolVal

	case settings.TypePassword:
		return huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(f.Label).
					Description(f.Description).
					EchoMode(huh.EchoModePassword).
					Value(&strVal),
			),
		), &strVal, nil

	default:
		strVal = current
		input := huh.NewInput().
			Title(f.Label).
			Description(f.Description).
			Value(&strVal)
		if f.Validate != nil {
			input = input.Validate(f.Validate)
		}
		return huh.NewForm(
			huh.NewGroup(input),
		), &strVal, nil
	}
}
