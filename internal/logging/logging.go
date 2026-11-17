package logging

import (
	"fmt"
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
)

type Level int

const (
	LevelNormal Level = iota
	LevelV
	LevelVV
	LevelVVV
)

type Logger struct {
	level  Level
	out    io.Writer
	errOut io.Writer

	info  lipgloss.Style
	ok    lipgloss.Style
	warn  lipgloss.Style
	err   lipgloss.Style
	debug lipgloss.Style
}

func New(level Level) *Logger {
	return &Logger{
		level:  level,
		out:    os.Stdout,
		errOut: os.Stderr,
		info:   lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true),
		ok:     lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true),
		warn:   lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true),
		err:    lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
		debug:  lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Bold(true),
	}
}

func LevelFromCount(n int) Level {
	switch {
	case n >= 3:
		return LevelVVV
	case n == 2:
		return LevelVV
	case n == 1:
		return LevelV
	default:
		return LevelNormal
	}
}

func (l *Logger) Info(format string, a ...any) {
	fmt.Fprintf(l.out, "%s %s\n", l.info.Render("[INFO] "), fmt.Sprintf(format, a...))
}

func (l *Logger) OK(format string, a ...any) {
	fmt.Fprintf(l.out, "%s %s\n", l.ok.Render("[OK]   "), fmt.Sprintf(format, a...))
}

func (l *Logger) Warn(format string, a ...any) {
	fmt.Fprintf(l.out, "%s %s\n", l.warn.Render("[WARN] "), fmt.Sprintf(format, a...))
}

func (l *Logger) Error(format string, a ...any) {
	fmt.Fprintf(l.errOut, "%s %s\n", l.err.Render("[ERR]  "), fmt.Sprintf(format, a...))
}

func (l *Logger) Debug(format string, a ...any) {
	if l.level >= LevelVVV {
		fmt.Fprintf(l.out, "%s %s\n", l.debug.Render("[DEBUG]"), fmt.Sprintf(format, a...))
	}
}

func (l *Logger) V(format string, a ...any) {
	if l.level >= LevelV {
		fmt.Fprintf(l.out, "%s %s\n", l.info.Render("[INFO] "), fmt.Sprintf(format, a...))
	}
}

func (l *Logger) VV(format string, a ...any) {
	if l.level >= LevelVV {
		fmt.Fprintf(l.out, "%s %s\n", l.info.Render("[INFO] "), fmt.Sprintf(format, a...))
	}
}

func (l *Logger) Level() Level { return l.level }
