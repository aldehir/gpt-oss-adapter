package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gpt-oss-adapter",
	Short: "A CLI application built with Cobra",
	Long:  "A longer description of your CLI application and what it does.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Hello from gpt-oss-adapter!")
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func main() {
	Execute()
}