use serde::{Deserialize, Serialize};
use tauri::{AppHandle, Emitter, Manager};
use tauri_plugin_shell::ShellExt;

use crate::sync::SyncStatus;

// ── Task count for taskbar badge ────────────────────────────────────────────

#[tauri::command]
pub async fn get_today_task_count() -> Result<u32, String> {
    // This is called from the frontend after querying the local SQLite.
    // The frontend passes the count back to update the taskbar badge.
    // Actual count is computed client-side from the SQL plugin.
    Ok(0)
}

#[tauri::command]
pub async fn update_taskbar_badge(app: AppHandle, count: u32) -> Result<(), String> {
    // On Windows, we'd update the taskbar overlay icon with the count.
    // On other platforms this is a no-op.
    #[cfg(target_os = "windows")]
    {
        update_windows_badge(&app, count);
    }

    let _ = app;
    let _ = count;
    Ok(())
}

#[cfg(target_os = "windows")]
fn update_windows_badge(_app: &AppHandle, _count: u32) {
    // Windows taskbar badge overlay via ITaskbarList3.
    // This requires COM initialization and the Windows Shell API.
    // The overlay icon is a small PNG rendered at runtime with the count.
    // For now, this is a stub — the full implementation uses:
    //   ITaskbarList3::SetOverlayIcon(hwnd, icon, description)
    // where `icon` is dynamically generated with the count text.
}

// ── Quick add task ──────────────────────────────────────────────────────────

#[derive(Debug, Serialize, Deserialize)]
pub struct QuickTask {
    pub title: String,
    pub planned_date: Option<String>,
}

#[tauri::command]
pub async fn quick_add_task(app: AppHandle, task: QuickTask) -> Result<String, String> {
    // Emit to frontend to handle the actual DB insert via SQL plugin
    app.emit("quick-add-task", &task).map_err(|e| e.to_string())?;

    // Return a generated UUID for the new task
    Ok(uuid::Uuid::new_v4().to_string())
}

// ── Sync commands ───────────────────────────────────────────────────────────

#[tauri::command]
pub async fn trigger_sync(app: AppHandle) -> Result<(), String> {
    crate::sync::trigger_manual_sync(&app).await;
    Ok(())
}

#[tauri::command]
pub async fn get_sync_status() -> Result<SyncStatus, String> {
    Ok(SyncStatus {
        syncing: false,
        last_synced_at: None,
        pending_mutations: 0,
        online: false,
    })
}

// ── Server URL config ───────────────────────────────────────────────────────

#[tauri::command]
pub async fn get_server_url() -> Result<String, String> {
    Ok(std::env::var("SEMPA_SERVER_URL").unwrap_or_default())
}

#[tauri::command]
pub async fn set_server_url(url: String) -> Result<(), String> {
    // Persisted via the store plugin from the frontend side.
    // This command is a fallback for setting the env var at runtime.
    // SAFETY: only called from the main thread via Tauri IPC.
    unsafe { std::env::set_var("SEMPA_SERVER_URL", &url) };
    Ok(())
}

// ── Window management ───────────────────────────────────────────────────────

#[tauri::command]
pub async fn create_widget_window(app: AppHandle) -> Result<(), String> {
    crate::windows::create_widget(&app).map_err(|e| e.to_string())
}

// ── Open a URL in the system default browser ────────────────────────────────

#[tauri::command]
pub async fn open_external(app: AppHandle, url: String) -> Result<(), String> {
    // Only allow web links — never arbitrary schemes (file://, etc.).
    let lower = url.to_ascii_lowercase();
    if !(lower.starts_with("http://") || lower.starts_with("https://")) {
        return Err("only http(s) URLs may be opened".into());
    }
    app.shell()
        .open(url, None)
        .map_err(|e| e.to_string())
}

#[tauri::command]
pub async fn create_pomodoro_widget_window(app: AppHandle) -> Result<(), String> {
    crate::windows::create_pomodoro_widget(&app).map_err(|e| e.to_string())
}

#[tauri::command]
pub async fn close_pomodoro_widget(app: AppHandle) -> Result<(), String> {
    if let Some(win) = app.get_webview_window("pomodoro") {
        win.close().map_err(|e| e.to_string())?;
    }
    Ok(())
}

#[tauri::command]
pub async fn show_reminder_popup(app: AppHandle) -> Result<(), String> {
    crate::windows::create_reminder_popup(&app).map_err(|e| e.to_string())
}

#[tauri::command]
pub async fn close_reminder_popup(app: AppHandle) -> Result<(), String> {
    if let Some(win) = app.get_webview_window("reminder") {
        win.close().map_err(|e| e.to_string())?;
    }
    Ok(())
}

#[tauri::command]
pub async fn create_sticky_note(
    app: AppHandle,
    note_id: String,
    x: f64,
    y: f64,
    width: f64,
    height: f64,
) -> Result<(), String> {
    crate::windows::create_sticky(&app, &note_id, x, y, width, height)
        .map_err(|e| e.to_string())
}

#[tauri::command]
pub async fn close_sticky_note(app: AppHandle, note_id: String) -> Result<(), String> {
    let label = format!("sticky-{}", note_id);
    if let Some(win) = app.get_webview_window(&label) {
        win.close().map_err(|e| e.to_string())?;
    }
    Ok(())
}

// ── Sticky note position persistence ────────────────────────────────────────

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StickyPosition {
    pub note_id: String,
    pub x: f64,
    pub y: f64,
    pub width: f64,
    pub height: f64,
}

#[tauri::command]
pub async fn save_sticky_positions(_positions: Vec<StickyPosition>) -> Result<(), String> {
    // Persisted via the store plugin from the frontend.
    Ok(())
}

#[tauri::command]
pub async fn get_sticky_positions() -> Result<Vec<StickyPosition>, String> {
    Ok(vec![])
}

/// Open the centered global Quick-Add window (also bound to the global shortcut
/// and the tray). Callable from the frontend.
#[tauri::command]
pub fn open_quick_add(app: AppHandle) -> Result<(), String> {
    crate::windows::create_quick_add(&app).map_err(|e| e.to_string())
}

// ── Secret Service keyring (desktop) ──────────────────────────────────────────
//
// Stores the app's bearer token (and any future device credential) in the OS
// secret store — Secret Service / libsecret on Linux, Credential Manager on
// Windows, Keychain on macOS — instead of plaintext localStorage. The frontend
// keeps a localStorage fallback so a host with no Secret Service daemon still
// works (never plaintext when the keyring is available). Under Flatpak this
// needs `--talk-name=org.freedesktop.secrets` in finish-args.

#[cfg(desktop)]
const KEYRING_SERVICE: &str = "ca.sempa.Sempa";

#[tauri::command]
pub fn secret_get(key: String) -> Result<Option<String>, String> {
    #[cfg(desktop)]
    {
        let entry = keyring::Entry::new(KEYRING_SERVICE, &key).map_err(|e| e.to_string())?;
        match entry.get_password() {
            Ok(p) => Ok(Some(p)),
            Err(keyring::Error::NoEntry) => Ok(None),
            Err(e) => Err(e.to_string()),
        }
    }
    #[cfg(not(desktop))]
    {
        let _ = key;
        Ok(None)
    }
}

#[tauri::command]
pub fn secret_set(key: String, value: String) -> Result<(), String> {
    #[cfg(desktop)]
    {
        let entry = keyring::Entry::new(KEYRING_SERVICE, &key).map_err(|e| e.to_string())?;
        entry.set_password(&value).map_err(|e| e.to_string())
    }
    #[cfg(not(desktop))]
    {
        let _ = (key, value);
        Ok(())
    }
}

#[tauri::command]
pub fn secret_delete(key: String) -> Result<(), String> {
    #[cfg(desktop)]
    {
        let entry = keyring::Entry::new(KEYRING_SERVICE, &key).map_err(|e| e.to_string())?;
        match entry.delete_credential() {
            Ok(()) | Err(keyring::Error::NoEntry) => Ok(()),
            Err(e) => Err(e.to_string()),
        }
    }
    #[cfg(not(desktop))]
    {
        let _ = key;
        Ok(())
    }
}

// ── Native window-button layout (Linux) ──────────────────────────────────────

/// Return the desktop's window-button layout so the custom titlebar can mirror
/// the system order/side (e.g. GNOME `appmenu:close`, KDE `:minimize,maximize,close`).
///
/// Reads GTK's `gtk-decoration-layout`, which the GTK port populates from the
/// active desktop (GNOME's `button-layout`, KDE's settings, etc.) — so it's
/// correct without talking to D-Bus directly. GTK must be touched on the main
/// thread, hence the hop. Returns `None` off Linux or if it can't be read; the
/// frontend then falls back to its configured default.
#[tauri::command]
pub fn window_decoration_layout(app: AppHandle) -> Option<String> {
    #[cfg(target_os = "linux")]
    {
        use std::sync::mpsc;
        let (tx, rx) = mpsc::channel();
        let dispatched = app.run_on_main_thread(move || {
            use gtk::prelude::GtkSettingsExt;
            let layout = gtk::Settings::default()
                .and_then(|s| s.gtk_decoration_layout())
                .map(|g| g.to_string());
            let _ = tx.send(layout);
        });
        if dispatched.is_err() {
            return None;
        }
        rx.recv_timeout(std::time::Duration::from_millis(500))
            .ok()
            .flatten()
    }
    #[cfg(not(target_os = "linux"))]
    {
        let _ = app;
        None
    }
}
