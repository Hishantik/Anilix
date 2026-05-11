package cmd

import (
	"os"

	"github.com/anilix/anilix/config"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "anilix",
	Short: "Anime streaming CLI",
	Long:  `Anilix - Stream anime from your terminal`,
	Run:   runInline,
}

func Execute() error {
	return rootCmd.Execute()
}

func Init() error {
	return config.Setup()
}

func runInline(cmd *cobra.Command, args []string) {
	// Placeholder for inline mode
}

func main() {
	if err := Init(); err != nil {
		os.Exit(1)
	}
	if err := Execute(); err != nil {
		os.Exit(1)
	}
}