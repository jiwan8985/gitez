package ui

import "fmt"

// ANSI escape codes — work on Windows 10 1511+ / Windows 11 PowerShell & CMD.
const (
	reset     = "\033[0m"
	bold      = "\033[1m"
	dim       = "\033[2m"
	colorRed  = "\033[31m"
	colorGreen = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue  = "\033[34m"
	colorCyan  = "\033[36m"
)

func Green(s string) string      { return colorGreen + s + reset }
func Red(s string) string        { return colorRed + s + reset }
func Yellow(s string) string     { return colorYellow + s + reset }
func Blue(s string) string       { return colorBlue + s + reset }
func Cyan(s string) string       { return colorCyan + s + reset }
func Bold(s string) string       { return bold + s + reset }
func Dim(s string) string        { return dim + s + reset }
func BoldGreen(s string) string  { return bold + colorGreen + s + reset }
func BoldRed(s string) string    { return bold + colorRed + s + reset }
func BoldCyan(s string) string   { return bold + colorCyan + s + reset }
func BoldYellow(s string) string { return bold + colorYellow + s + reset }

// Convenience printers
func Success(msg string) { fmt.Println(BoldGreen("✔ " + msg)) }
func Fail(msg string)    { fmt.Println(BoldRed("✘ " + msg)) }
func Info(msg string)    { fmt.Println(Cyan("  " + msg)) }
func Warn(msg string)    { fmt.Println(BoldYellow("⚠ " + msg)) }

// ColorXY colorises the two-character XY status field from `git status --short`.
func ColorXY(xy string) string {
	if len(xy) < 2 {
		return xy
	}
	return colorChar(string(xy[0])) + colorChar(string(xy[1]))
}

func colorChar(c string) string {
	switch c {
	case "M":
		return Yellow("M")
	case "A":
		return Green("A")
	case "D":
		return Red("D")
	case "R":
		return Cyan("R")
	case "C":
		return Cyan("C")
	case "U":
		return Red("U")
	case "?":
		return Blue("?")
	case " ":
		return " "
	default:
		return c
	}
}
