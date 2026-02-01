package logger

import (
"fmt"
"io"
"log"
"os"
"strings"
"sync"
"time"
)

// Level represents a log level
type Level int

const (
DEBUG Level = iota
INFO
WARN
ERROR
)

var levelNames = map[Level]string{
DEBUG: "DEBUG",
INFO:  "INFO",
WARN:  "WARN",
ERROR: "ERROR",
}

var levelColors = map[Level]string{
DEBUG: "\033[36m", // Cyan
INFO:  "\033[32m", // Green
WARN:  "\033[33m", // Yellow
ERROR: "\033[31m", // Red
}

const colorReset = "\033[0m"

// Logger is a custom logger with level support
type Logger struct {
level       Level
output      io.Writer
mu          sync.RWMutex
buffer      *RingBuffer
subscribers map[chan LogEntry]bool
subMu       sync.RWMutex
}

// LogEntry represents a single log entry
type LogEntry struct {
Timestamp time.Time `json:"timestamp"`
Level     string    `json:"level"`
Message   string    `json:"message"`
Source    string    `json:"source,omitempty"`
}

// RingBuffer stores the last N log entries
type RingBuffer struct {
entries []LogEntry
size    int
pos     int
mu      sync.RWMutex
}

// NewRingBuffer creates a new ring buffer
func NewRingBuffer(size int) *RingBuffer {
return &RingBuffer{
entries: make([]LogEntry, 0, size),
size:    size,
}
}

// Add adds an entry to the ring buffer
func (rb *RingBuffer) Add(entry LogEntry) {
rb.mu.Lock()
defer rb.mu.Unlock()

if len(rb.entries) < rb.size {
rb.entries = append(rb.entries, entry)
} else {
rb.entries[rb.pos] = entry
rb.pos = (rb.pos + 1) % rb.size
}
}

// GetAll returns all entries in chronological order
func (rb *RingBuffer) GetAll() []LogEntry {
rb.mu.RLock()
defer rb.mu.RUnlock()

if len(rb.entries) == 0 {
return []LogEntry{}
}

result := make([]LogEntry, len(rb.entries))
if len(rb.entries) < rb.size {
copy(result, rb.entries)
} else {
// Copy from pos to end, then from start to pos
n := copy(result, rb.entries[rb.pos:])
copy(result[n:], rb.entries[:rb.pos])
}
return result
}

var (
defaultLogger *Logger
once          sync.Once
)

// Init initializes the default logger
func Init(level Level) *Logger {
once.Do(func() {
defaultLogger = &Logger{
level:       level,
output:      os.Stdout,
buffer:      NewRingBuffer(1000), // Store last 1000 entries
subscribers: make(map[chan LogEntry]bool),
}
})
return defaultLogger
}

// Get returns the default logger
func Get() *Logger {
if defaultLogger == nil {
return Init(ERROR) // Default to ERROR level
}
return defaultLogger
}

// SetLevel sets the log level
func (l *Logger) SetLevel(level Level) {
l.mu.Lock()
defer l.mu.Unlock()
l.level = level
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() Level {
l.mu.RLock()
defer l.mu.RUnlock()
return l.level
}

// GetLevelName returns the name of the current log level
func (l *Logger) GetLevelName() string {
return levelNames[l.GetLevel()]
}

// log writes a log entry
func (l *Logger) log(level Level, source, format string, args ...interface{}) {
l.mu.RLock()
currentLevel := l.level
l.mu.RUnlock()

if level < currentLevel {
return
}

message := fmt.Sprintf(format, args...)
timestamp := time.Now()

entry := LogEntry{
Timestamp: timestamp,
Level:     levelNames[level],
Message:   message,
Source:    source,
}

// Add to buffer
l.buffer.Add(entry)

// Notify subscribers
l.notifySubscribers(entry)

// Write to output
levelName := levelNames[level]
color := levelColors[level]
timeStr := timestamp.Format("2006-01-02 15:04:05")

sourcePrefix := ""
if source != "" {
sourcePrefix = fmt.Sprintf("[%s] ", source)
}

logLine := fmt.Sprintf("%s%s%s [%s] %s%s\n", 
color, timeStr, colorReset, levelName, sourcePrefix, message)

l.output.Write([]byte(logLine))
}

// Debug logs a debug message
func (l *Logger) Debug(source, format string, args ...interface{}) {
l.log(DEBUG, source, format, args...)
}

// Info logs an info message
func (l *Logger) Info(source, format string, args ...interface{}) {
l.log(INFO, source, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(source, format string, args ...interface{}) {
l.log(WARN, source, format, args...)
}

// Error logs an error message
func (l *Logger) Error(source, format string, args ...interface{}) {
l.log(ERROR, source, format, args...)
}

// GetHistory returns all buffered log entries
func (l *Logger) GetHistory() []LogEntry {
return l.buffer.GetAll()
}

// Subscribe creates a new subscriber channel for log streaming
func (l *Logger) Subscribe() chan LogEntry {
l.subMu.Lock()
defer l.subMu.Unlock()

ch := make(chan LogEntry, 100)
l.subscribers[ch] = true
return ch
}

// Unsubscribe removes a subscriber
func (l *Logger) Unsubscribe(ch chan LogEntry) {
l.subMu.Lock()
defer l.subMu.Unlock()

delete(l.subscribers, ch)
close(ch)
}

// notifySubscribers sends log entry to all subscribers
func (l *Logger) notifySubscribers(entry LogEntry) {
l.subMu.RLock()
defer l.subMu.RUnlock()

for ch := range l.subscribers {
select {
case ch <- entry:
default:
// Skip if channel is full
}
}
}

// ParseLevel parses a log level string
func ParseLevel(s string) (Level, error) {
switch strings.ToUpper(s) {
case "DEBUG":
return DEBUG, nil
case "INFO":
return INFO, nil
case "WARN", "WARNING":
return WARN, nil
case "ERROR":
return ERROR, nil
default:
return ERROR, fmt.Errorf("invalid log level: %s", s)
}
}

// Convenience functions for default logger
func Debug(source, format string, args ...interface{}) {
Get().Debug(source, format, args...)
}

func Info(source, format string, args ...interface{}) {
Get().Info(source, format, args...)
}

func Warn(source, format string, args ...interface{}) {
Get().Warn(source, format, args...)
}

func Error(source, format string, args ...interface{}) {
Get().Error(source, format, args...)
}

// SetupStdLogger redirects standard library log to our logger
func SetupStdLogger() {
log.SetOutput(&stdLogWriter{logger: Get()})
log.SetFlags(0) // We'll handle formatting
}

type stdLogWriter struct {
logger *Logger
}

func (w *stdLogWriter) Write(p []byte) (n int, err error) {
msg := strings.TrimSpace(string(p))
if msg != "" {
w.logger.Info("", "%s", msg)
}
return len(p), nil
}
