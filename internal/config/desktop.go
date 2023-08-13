package config

import (
	"os"
	"regexp"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/demodesk/neko/pkg/types"
)

type Desktop struct {
	Display string

	Unminimize bool

	ScreenSize types.ScreenSize
}

func (Desktop) Init(cmd *cobra.Command) error {
	cmd.PersistentFlags().String("desktop.screen", "1280x720@30", "default screen size and framerate")
	if err := viper.BindPFlag("desktop.screen", cmd.PersistentFlags().Lookup("desktop.screen")); err != nil {
		return err
	}

	cmd.PersistentFlags().Bool("desktop.unminimize", true, "automatically unminimize window when it is minimized")
	if err := viper.BindPFlag("desktop.unminimize", cmd.PersistentFlags().Lookup("desktop.unminimize")); err != nil {
		return err
	}

	return nil
}

func (s *Desktop) Set() {
	// Display is provided by env variable
	s.Display = os.Getenv("DISPLAY")

	s.Unminimize = viper.GetBool("desktop.unminimize")

	s.ScreenSize = types.ScreenSize{
		Width:  1280,
		Height: 720,
		Rate:   30,
	}

	r := regexp.MustCompile(`([0-9]{1,4})x([0-9]{1,4})@([0-9]{1,3})`)
	res := r.FindStringSubmatch(viper.GetString("desktop.screen"))

	if len(res) > 0 {
		width, err1 := strconv.ParseInt(res[1], 10, 64)
		height, err2 := strconv.ParseInt(res[2], 10, 64)
		rate, err3 := strconv.ParseInt(res[3], 10, 64)

		if err1 == nil && err2 == nil && err3 == nil {
			s.ScreenSize.Width = int(width)
			s.ScreenSize.Height = int(height)
			s.ScreenSize.Rate = int16(rate)
		}
	}
}
