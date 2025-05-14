package utils

import (
        "fmt"
        "log"
        "os"
        "path/filepath"
)

// LogLevel defines the level of logging
type LogLevel int

const (
        // LogLevelDebug logs everything
        LogLevelDebug LogLevel = iota
        // LogLevelInfo logs info, warnings, and errors
        LogLevelInfo
        // LogLevelWarning logs warnings and errors
        LogLevelWarning
        // LogLevelError logs only errors
        LogLevelError
        // LogLevelNone disables logging
        LogLevelNone
)

var (
        // CurrentLogLevel is the current log level
        CurrentLogLevel = LogLevelInfo
        
        // LogToConsole controls whether to output logs to console
        LogToConsole = false
        
        // LogFile is the file to write logs to
        LogFile *os.File
)

// SetLogLevel sets the current log level
func SetLogLevel(level LogLevel) {
        CurrentLogLevel = level
}

// InitLogger initializes the logger
func InitLogger(level LogLevel, logToFile bool, logToConsole bool) error {
        CurrentLogLevel = level
        LogToConsole = logToConsole
        
        if logToFile {
                // Create logs directory if it doesn't exist
                err := os.MkdirAll("logs", 0755)
                if err != nil {
                        return fmt.Errorf("failed to create logs directory: %v", err)
                }
                
                // Open log file
                LogFile, err = os.OpenFile(filepath.Join("logs", "doucya.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
                if err != nil {
                        return fmt.Errorf("failed to open log file: %v", err)
                }
                
                // Set log output to file
                log.SetOutput(LogFile)
        }
        
        return nil
}

// Debug logs a debug message
func Debug(format string, args ...interface{}) {
        if CurrentLogLevel <= LogLevelDebug {
                logMessage("DEBUG", format, args...)
        }
}

// Info logs an info message
func Info(format string, args ...interface{}) {
        if CurrentLogLevel <= LogLevelInfo {
                logMessage("INFO", format, args...)
        }
}

// Warning logs a warning message
func Warning(format string, args ...interface{}) {
        if CurrentLogLevel <= LogLevelWarning {
                logMessage("WARNING", format, args...)
        }
}

// Error logs an error message
func Error(format string, args ...interface{}) {
        if CurrentLogLevel <= LogLevelError {
                logMessage("ERROR", format, args...)
        }
}

// logMessage logs a message with the given prefix
func logMessage(prefix, format string, args ...interface{}) {
        message := fmt.Sprintf(format, args...)
        formattedMessage := fmt.Sprintf("[%s] %s", prefix, message)
        
        if LogFile != nil {
                log.Println(formattedMessage)
        }
        
        if LogToConsole {
                fmt.Println(formattedMessage)
        }
}

// CloseLogger closes the logger
func CloseLogger() {
        if LogFile != nil {
                LogFile.Close()
        }
}