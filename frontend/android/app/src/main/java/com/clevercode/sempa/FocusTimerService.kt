package com.clevercode.sempa

import android.app.Service
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import androidx.core.app.NotificationCompat
import androidx.core.app.ServiceCompat
import androidx.glance.appwidget.updateAll
import com.clevercode.sempa.widget.SempaPomodoroWidget
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlin.math.ceil

/**
 * Foreground service that shows an ongoing "focus" notification with a live
 * countdown. It leans on Android's notification chronometer (setWhen + count-down)
 * so the timer renders and ticks **natively** — it keeps counting on the lock
 * screen and in the shade even while the WebView is suspended, with no JS running.
 *
 * The web Pomodoro timer (pomodoro.svelte.ts) remains the single source of truth.
 * This service mirrors its state and offers Pause/Done remote controls; taps are
 * relayed to JS live when the app is alive, and otherwise stashed in prefs for the
 * web layer to drain on next launch (consumePendingAction).
 */
class FocusTimerService : Service() {

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> {
                setActive(false); refreshFocusWidget()
                stopForegroundCompat(); stopSelf(); return START_NOT_STICKY
            }

            ACTION_BTN_DONE -> {
                relayAction("done")
                setActive(false); refreshFocusWidget()
                stopForegroundCompat(); stopSelf()
                return START_NOT_STICKY
            }

            ACTION_BTN_PAUSE -> {
                relayAction("pause")
                // Persist the pause (running=false, banked remaining) so BOTH the
                // notification AND the home-screen widget reflect it even with the
                // app closed; end=0 marks "no live countdown anchor".
                val p = loadState()
                val rem = remainingNow(p)
                saveState(p.title, p.phase, false, 0L, rem)
                startForegroundCompat(buildNotification(p.title, p.phase, false, 0L, rem))
                refreshFocusWidget()
                return START_STICKY
            }

            ACTION_BTN_RESUME -> {
                relayAction("resume")
                // Resume the countdown natively (new anchor = now + remaining) so the
                // widget/notification tick again even when the app isn't running; the
                // web store reconciles its own clock when next opened.
                val p = loadState()
                val end = System.currentTimeMillis() + p.remaining
                saveState(p.title, p.phase, true, end, p.remaining)
                startForegroundCompat(buildNotification(p.title, p.phase, true, end, p.remaining))
                refreshFocusWidget()
                return START_STICKY
            }

            else -> {
                // ACTION_SHOW — create or update the notification from the extras.
                val title = intent?.getStringExtra(EXTRA_TITLE) ?: "Focus"
                val phase = intent?.getStringExtra(EXTRA_PHASE) ?: "Focus"
                val running = intent?.getBooleanExtra(EXTRA_RUNNING, true) ?: true
                val endTime = intent?.getLongExtra(EXTRA_END, 0L) ?: 0L
                val remaining = intent?.getLongExtra(EXTRA_REMAINING, 0L) ?: 0L
                saveState(title, phase, running, endTime, remaining)
                startForegroundCompat(buildNotification(title, phase, running, endTime, remaining))
                refreshFocusWidget()
                return START_STICKY
            }
        }
    }

    private fun buildNotification(
        title: String, phase: String, running: Boolean, endTime: Long, remainingMs: Long,
    ): android.app.Notification {
        val tap = PendingIntent.getActivity(
            this, 0,
            Intent(this, MainActivity::class.java).apply {
                flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP
            },
            PendingIntent.FLAG_IMMUTABLE,
        )

        val b = NotificationCompat.Builder(this, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.ic_lock_idle_alarm)
            .setContentTitle(title)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setSilent(true)
            .setContentIntent(tap)
            .setVisibility(NotificationCompat.VISIBILITY_PUBLIC)
            .setCategory(NotificationCompat.CATEGORY_STOPWATCH)
            .setForegroundServiceBehavior(NotificationCompat.FOREGROUND_SERVICE_IMMEDIATE)

        if (running && endTime > System.currentTimeMillis()) {
            // Native live countdown — no JS required.
            b.setUsesChronometer(true)
            b.setWhen(endTime)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) b.setChronometerCountDown(true)
            b.setContentText(phase)
            b.addAction(0, "Pause", servicePI(ACTION_BTN_PAUSE, 1))
        } else {
            b.setUsesChronometer(false)
            val mins = if (remainingMs > 0) ceil(remainingMs / 60000.0).toInt() else 0
            b.setContentText(if (mins > 0) "Paused · ${mins}m left" else "Paused")
            b.addAction(0, "Resume", servicePI(ACTION_BTN_RESUME, 2))
        }
        b.addAction(0, "Done", servicePI(ACTION_BTN_DONE, 3))
        return b.build()
    }

    private fun servicePI(action: String, req: Int): PendingIntent {
        val i = Intent(this, FocusTimerService::class.java).setAction(action)
        return PendingIntent.getService(
            this, req, i,
            PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
    }

    private fun startForegroundCompat(n: android.app.Notification) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.UPSIDE_DOWN_CAKE) {
            ServiceCompat.startForeground(this, NOTIF_ID, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_SPECIAL_USE)
        } else if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            ServiceCompat.startForeground(this, NOTIF_ID, n, 0)
        } else {
            startForeground(NOTIF_ID, n)
        }
    }

    private fun stopForegroundCompat() {
        ServiceCompat.stopForeground(this, ServiceCompat.STOP_FOREGROUND_REMOVE)
    }

    // ── Action relay + state persistence ────────────────────────────────────
    private fun relayAction(action: String) {
        // Stash for the web layer to drain on next launch...
        prefs().edit().putString(KEY_PENDING, action).putLong(KEY_PENDING_TS, System.currentTimeMillis()).apply()
        // ...and deliver live if the app is currently running (clears the stash).
        FocusTimerPlugin.emit(this, action)
    }

    private data class State(val title: String, val phase: String, val running: Boolean, val endTime: Long, val remaining: Long)
    private fun saveState(title: String, phase: String, running: Boolean, endTime: Long, remaining: Long) {
        prefs().edit()
            .putString(KEY_TITLE, title).putString(KEY_PHASE, phase)
            .putBoolean(KEY_RUNNING, running).putLong(KEY_END, endTime).putLong(KEY_REMAIN, remaining)
            .putBoolean(KEY_ACTIVE, true) // a saved session is, by definition, active
            .apply()
    }
    private fun setActive(active: Boolean) { prefs().edit().putBoolean(KEY_ACTIVE, active).apply() }

    /** Re-render the home-screen Pomodoro widget after a state change. */
    private fun refreshFocusWidget() {
        CoroutineScope(Dispatchers.IO).launch {
            try { SempaPomodoroWidget().updateAll(applicationContext) } catch (_: Exception) {}
        }
    }
    private fun loadState(): State {
        val p = prefs()
        return State(
            p.getString(KEY_TITLE, "Focus") ?: "Focus",
            p.getString(KEY_PHASE, "Focus") ?: "Focus",
            p.getBoolean(KEY_RUNNING, true),
            p.getLong(KEY_END, 0L),
            p.getLong(KEY_REMAIN, 0L),
        )
    }
    private fun remainingNow(s: State): Long =
        if (s.endTime > 0L) maxOf(0L, s.endTime - System.currentTimeMillis()) else s.remaining

    private fun prefs() = getSharedPreferences(PREFS, Context.MODE_PRIVATE)

    companion object {
        const val PREFS = "sempa_focus_timer"
        const val CHANNEL_ID = "focus_timer"
        const val NOTIF_ID = 4_201

        const val ACTION_SHOW = "com.clevercode.sempa.focus.SHOW"
        const val ACTION_STOP = "com.clevercode.sempa.focus.STOP"
        const val ACTION_BTN_PAUSE = "com.clevercode.sempa.focus.PAUSE"
        const val ACTION_BTN_RESUME = "com.clevercode.sempa.focus.RESUME"
        const val ACTION_BTN_DONE = "com.clevercode.sempa.focus.DONE"

        const val EXTRA_TITLE = "title"
        const val EXTRA_PHASE = "phase"
        const val EXTRA_RUNNING = "running"
        const val EXTRA_END = "endTime"
        const val EXTRA_REMAINING = "remaining"

        const val KEY_PENDING = "pending_action"
        const val KEY_PENDING_TS = "pending_ts"
        // A task id stashed by the widget's "start this task" tap, drained by JS on
        // next foreground (consumePendingStart) so the web store owns the actual start.
        const val KEY_PENDING_START = "pending_start_task"
        // Whether a focus session is currently active (drives the widget's active vs
        // idle layout). Read by SempaPomodoroWidget; written here.
        const val KEY_ACTIVE = "s_active"
        // Read by SempaPomodoroWidget to render the active session, so non-private.
        const val KEY_TITLE = "s_title"
        const val KEY_PHASE = "s_phase"
        const val KEY_RUNNING = "s_running"
        const val KEY_END = "s_end"
        const val KEY_REMAIN = "s_remain"
    }
}
