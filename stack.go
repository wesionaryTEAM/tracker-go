package tracker

import (
	"runtime"
	"strconv"
	"strings"
)

// captureStack captures the current goroutine's call stack.
// skip is the number of extra frames to skip above captureStack itself —
// pass 1 to skip the direct caller, 2 to skip the caller's caller, etc.
// Runtime frames (files under GOROOT) are filtered out.
func captureStack(skip int) []StackFrame {
	pcs := make([]uintptr, 64)
	// +2: skip runtime.Callers itself (0) and captureStack (1)
	n := runtime.Callers(skip+2, pcs)
	goroot := runtime.GOROOT()
	var frames []StackFrame
	iter := runtime.CallersFrames(pcs[:n])
	for {
		f, more := iter.Next()
		if !strings.HasPrefix(f.File, goroot) {
			frames = append(frames, StackFrame{
				File: f.File,
				Line: f.Line,
				Col:  0,
				Fn:   f.Function,
			})
		}
		if !more {
			break
		}
	}
	return frames
}

// parsePanicStack parses the output of debug.Stack() into []StackFrame.
// Runtime frames are filtered out.
func parsePanicStack(b []byte) []StackFrame {
	lines := strings.Split(string(b), "\n")
	goroot := runtime.GOROOT()
	var frames []StackFrame
	// Line 0 is "goroutine N [running]:" — skip it.
	// Subsequent pairs: function line, then indented file:line line.
	i := 1
	for i < len(lines) {
		fnLine := strings.TrimSpace(lines[i])
		i++
		// A function line contains "(" — file lines are tab-indented paths.
		if fnLine == "" || !strings.Contains(fnLine, "(") {
			continue
		}
		if i >= len(lines) {
			break
		}
		fileLine := strings.TrimSpace(lines[i])
		i++
		// fileLine format: "/path/to/file.go:42 +0x123"
		parts := strings.Fields(fileLine)
		if len(parts) == 0 {
			continue
		}
		fileAndLine := parts[0]
		colonIdx := strings.LastIndex(fileAndLine, ":")
		if colonIdx < 0 {
			continue
		}
		file := fileAndLine[:colonIdx]
		lineNum, _ := strconv.Atoi(fileAndLine[colonIdx+1:])
		if strings.HasPrefix(file, goroot) {
			continue
		}
		// Strip argument list from function name.
		fnName := fnLine
		if idx := strings.Index(fnLine, "("); idx > 0 {
			fnName = fnLine[:idx]
		}
		frames = append(frames, StackFrame{
			File: file,
			Line: lineNum,
			Col:  0,
			Fn:   fnName,
		})
	}
	return frames
}
