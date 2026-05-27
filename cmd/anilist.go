package cmd

import (
	"fmt"
	"os"

	"github.com/hishantik/anilix/auth"
	"github.com/spf13/cobra"
)

var anilistCmd = &cobra.Command{
	Use:   "anilist",
	Short: "Manage AniList account integration",
	Long:  `Connect your AniList account to track anime watching progress automatically.`,
}

var anilistLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to AniList",
	Long:  `Open a browser to authorize Anilix with your AniList account.`,
	Run: func(cmd *cobra.Command, args []string) {
		manual, _ := cmd.Flags().GetBool("manual")
		var err error
		if manual {
			err = auth.LoginManual()
		} else {
			err = auth.Login()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Login failed: %v\n", err)
			os.Exit(1)
		}
	},
}

var anilistLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out of AniList",
	Long:  `Remove stored AniList credentials from this device.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := auth.Logout(); err != nil {
			fmt.Fprintf(os.Stderr, "Logout failed: %v\n", err)
			os.Exit(1)
		}
	},
}

var anilistStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show AniList login status",
	Long:  `Display the currently logged-in AniList user, if any.`,
	Run: func(cmd *cobra.Command, args []string) {
		if auth.IsLoggedIn() {
			fmt.Printf("Logged in as %s\n", auth.GetUsername())
		} else {
			fmt.Println("Not logged in to AniList.")
			fmt.Println("Run 'anilix anilist login' to connect your account.")
		}
	},
}

func init() {
	anilistLoginCmd.Flags().Bool("manual", false, "Manual login (copy-paste URL instead of opening browser)")
	anilistCmd.AddCommand(anilistLoginCmd)
	anilistCmd.AddCommand(anilistLogoutCmd)
	anilistCmd.AddCommand(anilistStatusCmd)
	rootCmd.AddCommand(anilistCmd)
}
