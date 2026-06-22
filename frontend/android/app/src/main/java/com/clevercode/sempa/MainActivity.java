package com.clevercode.sempa;

import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.content.Intent;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import androidx.core.content.pm.ShortcutInfoCompat;
import androidx.core.content.pm.ShortcutManagerCompat;
import androidx.core.graphics.drawable.IconCompat;
import java.util.Arrays;
import android.os.VibrationEffect;
import android.os.Vibrator;
import android.os.VibratorManager;
import android.webkit.JavascriptInterface;

import com.clevercode.sempa.widget.WidgetRefreshWorker;
import com.getcapacitor.BridgeActivity;

public class MainActivity extends BridgeActivity {

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        registerPlugin(WidgetBridgePlugin.class);
        registerPlugin(FocusTimerPlugin.class);
        super.onCreate(savedInstanceState);
        createNotificationChannels();
        WidgetRefreshWorker.Companion.enqueuePeriodicRefresh(this);
        registerShortcuts();

        // Expose haptics to WebView
        getBridge().getWebView().addJavascriptInterface(new HapticsInterface(), "SempaHaptics");

        // Launched from the home-screen Pomodoro widget's "start this task" tap:
        // stash the id for JS to drain on init (the web store owns the actual start).
        handleStartFocus(getIntent(), false);
        // Shared into Sempa from another app (ACTION_SEND): stash for JS to turn
        // into a new task on init.
        handleSharedText(getIntent());
    }

    @Override
    protected void onNewIntent(Intent intent) {
        super.onNewIntent(intent);
        setIntent(intent);
        // App already running → deliver live as well as stashing.
        handleStartFocus(intent, true);
        handleSharedText(intent);
    }

    /** A Share-to-Sempa intent (text/*): stash the text + subject; JS drains it on
     *  init / next foreground and opens a prefilled new-task composer. */
    private void handleSharedText(Intent intent) {
        if (intent == null || !Intent.ACTION_SEND.equals(intent.getAction())) return;
        String type = intent.getType();
        if (type == null || !type.startsWith("text/")) return;
        CharSequence text = intent.getCharSequenceExtra(Intent.EXTRA_TEXT);
        if (text == null || text.length() == 0) return;
        CharSequence subject = intent.getCharSequenceExtra(Intent.EXTRA_SUBJECT);
        getSharedPreferences("sempa_share", MODE_PRIVATE).edit()
            .putString("text", text.toString())
            .putString("subject", subject == null ? "" : subject.toString())
            .apply();
        intent.removeExtra(Intent.EXTRA_TEXT); // don't reprocess on rotation/relaunch
    }

    /** Long-press launcher icon → New task / Plan today / Daily shutdown. Each opens
     *  a sempa:// deep link the web layer routes (initMobileDeepLinks). Dynamic (not
     *  static XML) so they attach to the app regardless of which icon alias is the
     *  active launcher. */
    private void registerShortcuts() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.N_MR1) return; // ShortcutManager is API 25+
        ShortcutInfoCompat newTask = buildShortcut("new", "New task", "New task",
            R.drawable.ic_shortcut_new, "com.clevercode.sempa://new");
        ShortcutInfoCompat plan = buildShortcut("plan", "Plan today", "Plan today",
            R.drawable.ic_shortcut_plan, "com.clevercode.sempa://plan");
        ShortcutInfoCompat shutdown = buildShortcut("shutdown", "Shutdown", "Daily shutdown",
            R.drawable.ic_shortcut_shutdown, "com.clevercode.sempa://shutdown");
        try {
            ShortcutManagerCompat.setDynamicShortcuts(this, Arrays.asList(newTask, plan, shutdown));
        } catch (Exception ignored) { /* some launchers cap/forbid shortcuts */ }
    }

    private ShortcutInfoCompat buildShortcut(String id, String shortLabel, String longLabel, int iconRes, String uri) {
        Intent intent = new Intent(this, MainActivity.class)
            .setAction(Intent.ACTION_VIEW)
            .setData(Uri.parse(uri));
        return new ShortcutInfoCompat.Builder(this, id)
            .setShortLabel(shortLabel)
            .setLongLabel(longLabel)
            .setIcon(IconCompat.createWithResource(this, iconRes))
            .setIntent(intent)
            .build();
    }

    private void handleStartFocus(Intent intent, boolean emitLive) {
        if (intent == null) return;
        String taskId = intent.getStringExtra("startFocusTaskId");
        if (taskId == null || taskId.isEmpty()) return;
        getSharedPreferences(FocusTimerService.PREFS, MODE_PRIVATE)
            .edit().putString(FocusTimerService.KEY_PENDING_START, taskId).apply();
        intent.removeExtra("startFocusTaskId"); // don't reprocess on rotation/relaunch
        if (emitLive) FocusTimerPlugin.Companion.emitStart(this, taskId);
    }

    private void createNotificationChannels() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            NotificationManager manager = getSystemService(NotificationManager.class);
            if (manager == null) return;

            NotificationChannel reminders = new NotificationChannel(
                "reminders",
                "Task Reminders",
                NotificationManager.IMPORTANCE_HIGH
            );
            reminders.setDescription("Reminders for upcoming and overdue tasks");
            reminders.enableVibration(true);
            manager.createNotificationChannel(reminders);

            NotificationChannel sync = new NotificationChannel(
                "sync",
                "Sync Status",
                NotificationManager.IMPORTANCE_LOW
            );
            sync.setDescription("Integration sync notifications");
            manager.createNotificationChannel(sync);

            // Ongoing focus-timer notification: silent, low-importance, no vibration
            // — it's a persistent status surface, not an alert.
            NotificationChannel focus = new NotificationChannel(
                "focus_timer",
                "Focus Timer",
                NotificationManager.IMPORTANCE_LOW
            );
            focus.setDescription("The running focus timer, shown while a session is active");
            focus.enableVibration(false);
            focus.setShowBadge(false);
            manager.createNotificationChannel(focus);
        }
    }

    /** JavascriptInterface exposed to the WebView for native haptic feedback. */
    private class HapticsInterface {

        @JavascriptInterface
        public void click() {
            vibrate(VibrationEffect.EFFECT_CLICK);
        }

        @JavascriptInterface
        public void tick() {
            vibrate(VibrationEffect.EFFECT_TICK);
        }

        @JavascriptInterface
        public void heavyClick() {
            vibrate(VibrationEffect.EFFECT_HEAVY_CLICK);
        }

        private void vibrate(int effectId) {
            if (Build.VERSION.SDK_INT < Build.VERSION_CODES.Q) return;

            Vibrator vibrator;
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                VibratorManager vm = getSystemService(VibratorManager.class);
                vibrator = vm != null ? vm.getDefaultVibrator() : null;
            } else {
                vibrator = (Vibrator) getSystemService(VIBRATOR_SERVICE);
            }

            if (vibrator != null && vibrator.hasVibrator()) {
                vibrator.vibrate(VibrationEffect.createPredefined(effectId));
            }
        }
    }
}
