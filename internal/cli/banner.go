package cli

import (
	"fmt"
	"os"
)

const bannerText = `███████╗ ██████╗ ██████╗  ██████╗ ███████╗
██╔════╝██╔═══██╗██╔══██╗██╔════╝ ██╔════╝
█████╗  ██║   ██║██████╔╝██║  ███╗█████╗
██╔══╝  ██║   ██║██╔══██╗██║   ██║██╔══╝
██║     ╚██████╔╝██║  ██║╚██████╔╝███████╗
╚═╝      ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝`

const tagline = "Scaffold projects you actually want to build"

const (
	colorReset  = "\033[0m"
	colorBlue   = "\033[38;5;33m"
	colorOrange = "\033[38;5;208m"
	colorBold   = "\033[1m"
)

// useColor returns true if stdout is a TTY and NO_COLOR is not set.
func useColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

// printBanner writes the ASCII banner and tagline to stdout.
func printBanner() {
	if useColor() {
		fmt.Printf("%s%s%s%s\n", colorBlue, colorBold, bannerText, colorReset)
		fmt.Printf("  %s%s%s\n\n", colorOrange, tagline, colorReset)
	} else {
		fmt.Println(bannerText)
		fmt.Println("  " + tagline)
		fmt.Println()
	}
}

// buildVersionTemplate returns the cobra version template string, with ANSI
// color if stdout is a TTY. Called once at init time.
func buildVersionTemplate() string {
	if useColor() {
		return colorBlue + colorBold + bannerText + colorReset + "\n" +
			"  " + colorOrange + "forge v{{.Version}}" + colorReset + "\n"
	}
	return bannerText + "\n  forge v{{.Version}}\n"
}
