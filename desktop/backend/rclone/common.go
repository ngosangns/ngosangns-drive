package rclone

import (
	"context"
	beConfig "desktop/backend/config"
	"desktop/backend/utils"
	"os"
	"runtime/pprof"

	env "github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/filter"
	fslog "github.com/rclone/rclone/fs/log"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/rcserver"
	"github.com/rclone/rclone/lib/atexit"
	"github.com/rclone/rclone/lib/terminal"
)

func InitConfig(ctx context.Context) (context.Context, error) {
	// Set the global options from the flags
	err := fs.GlobalOptionsInit()

	if utils.HandleError(err, "Failed to initialise global options", nil, nil) != nil {
		return nil, err
	}

	// Start the logger
	fslog.InitLogging()

	// Load the config
	// config.SetConfigPath(filepath.Join(dir, "rclone.conf"))
	configfile.Install()

	// Start accounting
	stats := accounting.Stats(ctx)
	stats.ResetCounters()
	accounting.Start(ctx)

	// Initialize the global config
	fsConfig := fs.GetConfig(ctx)
	fsConfig.CheckSum = true
	fsConfig.Progress = true
	fsConfig.TrackRenames = true
	fsConfig.Metadata = true
	fsConfig.UseServerModTime = true

	// Fix case
	fsConfig.NoUnicodeNormalization = false
	fsConfig.IgnoreCaseSync = true
	fsConfig.FixCase = true

	// Configure console
	if fsConfig.NoConsole {
		// Hide the console window
		terminal.HideConsole()
	} else {
		// Enable color support on stdout if possible.
		// This enables virtual terminal processing on Windows 10,
		// adding native support for ANSI/VT100 escape sequences.
		terminal.EnableColorsStdout()
	}

	// Write the args for debug purposes
	fs.Debugf("rclone", "Version %q starting with parameters %q", fs.Version, os.Args)

	// Inform user about systemd log support now that we have a logger
	if fslog.Opt.LogSystemdSupport {
		fs.Debugf("rclone", "systemd logging support activated")
	}

	// Start the metrics server if configured
	_, err = rcserver.MetricsStart(ctx, &rc.Opt)
	if utils.HandleError(err, "Failed to start metrics server", nil, nil) != nil {
		return nil, err
	}

	err = fsConfig.StatsLogLevel.Set(fs.LogLevelInfo.String()) // EMERGENCY ALERT CRITICAL ERROR WARNING NOTICE INFO DEBUG
	if utils.HandleError(err, "Failed to set stats log level", nil, nil) != nil {
		return nil, err
	}

	err = fsConfig.LogLevel.Set(fs.LogLevelInfo.String())
	if utils.HandleError(err, "Failed to set log level", nil, nil) != nil {
		return nil, err
	}

	// Setup the default filters
	filterOpts := filter.Options{
		MinAge:  fs.DurationOff, // These have to be set here as the options are parsed once before the defaults are set
		MaxAge:  fs.DurationOff,
		MinSize: fs.SizeSuffix(-1),
		MaxSize: fs.SizeSuffix(-1),
	}
	filterConfig, err := filter.NewFilter(&filterOpts)
	if utils.HandleError(err, "Failed to load filter file", nil, nil) != nil {
		return nil, err
	}
	ctx = filter.ReplaceConfig(ctx, filterConfig)

	// Setup CPU profiling if desired
	cpuProfile := ""
	// cpuProfile := "Debugging"
	if cpuProfile != "" {
		fs.Infof(nil, "Creating CPU profile %q\n", cpuProfile)
		f, err := os.Create(cpuProfile)
		if utils.HandleError(err, "", nil, nil) != nil {
			err = fs.CountError(err)
			return nil, err
		}
		err = pprof.StartCPUProfile(f)
		if utils.HandleError(err, "", nil, nil) != nil {
			err = fs.CountError(err)
			return nil, err
		}
		atexit.Register(func() {
			pprof.StopCPUProfile()
			err := f.Close()
			if utils.HandleError(err, "", nil, nil) != nil {
				fs.CountError(err)
			}
		})
	}

	// Setup memory profiling if desired
	memProfile := ""
	// memProfile := "Debugging"
	if memProfile != "" {
		atexit.Register(func() {
			fs.Infof(nil, "Saving Memory profile %q\n", memProfile)
			f, err := os.Create(memProfile)
			if utils.HandleError(err, "", nil, nil) != nil {
				fs.CountError(err)
			}
			err = pprof.WriteHeapProfile(f)
			if utils.HandleError(err, "", nil, nil) != nil {
				fs.CountError(err)
			}
			err = f.Close()
			if utils.HandleError(err, "", nil, nil) != nil {
				fs.CountError(err)
			}
		})
	}

	return ctx, nil
}

func LoadConfigFromEnv() (*beConfig.Config, error) {
	// Load the .env file
	err := godotenv.Load(".env")
	if err != nil {
		return nil, err
	}

	// Parse environment variables into the struct
	var config beConfig.Config
	if err := env.Parse(&config); err != nil {
		return nil, err
	}

	return &config, nil
}