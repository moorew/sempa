mod commands;
mod db;
mod sync;
mod tray;
mod windows;

use tauri::Manager;
use std::io::Write;

/// Write a line to a startup log file in a location that's always writable,
/// so a launch failure is never silent (Windows release builds have no console).
/// Writes to the OS temp dir, which exists and is writable on every platform.
pub(crate) fn startup_log(msg: &str) {
    let path = std::env::temp_dir().join("sempa-startup.log");
    if let Ok(mut f) = std::fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(&path)
    {
        let _ = writeln!(f, "[sempa] {msg}");
    }
    // Also emit to stderr for dev/console builds.
    eprintln!("[sempa] {msg}");
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    // Install a panic hook before anything else so any startup panic (e.g. a
    // bad plugin config, a missing WebView2 runtime) is recorded to disk rather
    // than vanishing silently on a windowed Windows build.
    std::panic::set_hook(Box::new(|info| {
        startup_log(&format!("PANIC: {info}"));
    }));

    startup_log("starting up");

    let mut builder = tauri::Builder::default();

    // Single-instance MUST be the first plugin registered: it acquires the lock
    // before anything else, so a second launch is intercepted and its argv (incl.
    // any sempa:// deep link) is forwarded to the running instance. Desktop-only —
    // there is no mobile target for this Tauri build (Android ships via Capacitor).
    // The plugin's `deep-link` feature wires forwarded URLs into the deep-link
    // plugin's on-open-url event automatically; this callback just surfaces the
    // existing window so a re-launch (or a .desktop action) focuses Sempa.
    #[cfg(desktop)]
    {
        builder = builder.plugin(tauri_plugin_single_instance::init(|app, _argv, _cwd| {
            if let Some(win) = app.get_webview_window("main") {
                let _ = win.show();
                let _ = win.unminimize();
                let _ = win.set_focus();
            }
        }));
    }

    // Global quick-add shortcut (desktop). The handler opens the centered
    // Quick-Add window on key-down; the shortcut itself is registered in setup.
    #[cfg(desktop)]
    {
        use tauri_plugin_global_shortcut::ShortcutState;
        builder = builder.plugin(
            tauri_plugin_global_shortcut::Builder::new()
                .with_handler(|app, _shortcut, event| {
                    if matches!(event.state(), ShortcutState::Pressed) {
                        if let Err(e) = crate::windows::create_quick_add(app) {
                            startup_log(&format!("global quick-add failed: {e}"));
                        }
                    }
                })
                .build(),
        );
    }

    let result = builder
        .plugin(tauri_plugin_deep_link::init())
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_notification::init())
        .plugin(
            tauri_plugin_sql::Builder::new()
                .add_migrations("sqlite:sempa.db", db::get_migrations())
                .build(),
        )
        .plugin(tauri_plugin_autostart::init(
            tauri_plugin_autostart::MacosLauncher::LaunchAgent,
            Some(vec!["--minimized"]),
        ))
        .plugin(tauri_plugin_store::Builder::new().build())
        .setup(|app| {
            startup_log("setup: begin");

            // Register the sempa:// scheme with the running binary at runtime. For
            // installed packages the .desktop MimeType already declares it; this
            // covers dev runs and the portable AppImage (where no .desktop is
            // installed). Best-effort: a sandboxed/locked-down environment may
            // refuse, which must not block startup.
            #[cfg(desktop)]
            {
                use tauri_plugin_deep_link::DeepLinkExt;
                if let Err(e) = app.deep_link().register_all() {
                    startup_log(&format!("setup: deep-link register_all failed (non-fatal): {e}"));
                }
            }

            // Register the global quick-add shortcut. Best-effort: under Wayland
            // this needs the GlobalShortcuts portal (compositor-dependent), so a
            // failure here must not block startup — the tray + window still work.
            #[cfg(desktop)]
            {
                use tauri_plugin_global_shortcut::GlobalShortcutExt;
                if let Err(e) = app.global_shortcut().register("CommandOrControl+Shift+Space") {
                    startup_log(&format!(
                        "setup: global quick-add shortcut not registered (non-fatal): {e}"
                    ));
                }
            }

            // Initialize the system tray. A tray failure must not take the
            // whole app down — log it and continue so the window still opens.
            if let Err(e) = tray::create_tray(app.handle()) {
                startup_log(&format!("setup: tray creation failed (non-fatal): {e}"));
            }

            // Run database migrations on startup
            let app_handle = app.handle().clone();
            tauri::async_runtime::spawn(async move {
                if let Err(e) = db::run_migrations(&app_handle).await {
                    eprintln!("Migration error: {e}");
                }
            });

            // Start the background sync engine
            let app_handle = app.handle().clone();
            tauri::async_runtime::spawn(async move {
                sync::start_sync_loop(app_handle).await;
            });

            // DevTools open automatically in debug builds; release builds ship
            // without them. (A temporary release auto-open was used to diagnose
            // the sync issue — removed now that it's fixed.)
            #[cfg(debug_assertions)]
            if let Some(win) = app.get_webview_window("main") {
                win.open_devtools();
            }

            // Check if launched with --minimized flag (startup boot)
            let minimized = std::env::args().any(|a| a == "--minimized");
            if minimized {
                if let Some(win) = app.get_webview_window("main") {
                    let _ = win.hide();
                }
            }

            // Sempa Dock appliance mode (`--dock` arg or SEMPA_DOCK=1): the Pi
            // kiosk launches the same binary fullscreen and lands on /dock. The
            // cursor is hidden by the /dock CSS; on the appliance the device is
            // already paired (token present) so the auth gate doesn't redirect.
            let dock_mode = std::env::args().any(|a| a == "--dock")
                || std::env::var("SEMPA_DOCK").is_ok();
            if dock_mode {
                if let Some(win) = app.get_webview_window("main") {
                    let _ = win.set_fullscreen(true);
                    let _ = win.set_decorations(false);
                    let _ = win.eval("window.location.replace('/dock')");
                }
            }

            startup_log("setup: complete");
            Ok(())
        })
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                if window.label() == "main" {
                    // Windows/macOS: minimize to tray instead of closing — the tray
                    // (and native toast) keep background reminders alive. Linux:
                    // actually quit. GNOME/KDE often don't surface a
                    // StatusNotifierItem tray, so hiding would strand the app with
                    // no way to resurface it (and single-instance would just
                    // re-focus the hidden window on the next launch). Letting the
                    // close proceed exits the app.
                    #[cfg(not(target_os = "linux"))]
                    {
                        api.prevent_close();
                        let _ = window.hide();
                    }
                    #[cfg(target_os = "linux")]
                    let _ = api; // close proceeds → app exits
                }
            }
        })
        .invoke_handler(tauri::generate_handler![
            commands::get_today_task_count,
            commands::quick_add_task,
            commands::trigger_sync,
            commands::get_sync_status,
            commands::get_server_url,
            commands::set_server_url,
            commands::create_widget_window,
            commands::open_external,
            commands::show_reminder_popup,
            commands::close_reminder_popup,
            commands::create_sticky_note,
            commands::close_sticky_note,
            commands::save_sticky_positions,
            commands::get_sticky_positions,
            commands::update_taskbar_badge,
            commands::window_decoration_layout,
            commands::open_quick_add,
            commands::secret_get,
            commands::secret_set,
            commands::secret_delete,
        ])
        .run(tauri::generate_context!());

    if let Err(e) = result {
        startup_log(&format!("FATAL: tauri runtime exited with error: {e}"));
        std::process::exit(1);
    }
}
