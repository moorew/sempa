package com.clevercode.sempa.widget

import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.SystemClock
import android.widget.RemoteViews
import androidx.compose.runtime.Composable
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.glance.*
import androidx.glance.appwidget.AndroidRemoteViews
import com.clevercode.sempa.R
import androidx.glance.action.ActionParameters
import androidx.glance.action.actionParametersOf
import androidx.glance.action.actionStartActivity
import androidx.glance.action.clickable
import androidx.glance.appwidget.GlanceAppWidget
import androidx.glance.appwidget.GlanceAppWidgetReceiver
import androidx.glance.appwidget.action.ActionCallback
import androidx.glance.appwidget.action.actionRunCallback
import androidx.glance.appwidget.cornerRadius
import androidx.glance.appwidget.provideContent
import androidx.glance.layout.*
import androidx.glance.text.*
import androidx.glance.unit.ColorProvider
import com.clevercode.sempa.FocusTimerService
import com.clevercode.sempa.MainActivity
import kotlin.math.ceil

/**
 * Home-screen Pomodoro/focus widget. Two states, driven entirely by SharedPreferences
 * (no JS required while the app is closed):
 *
 *  • ACTIVE — reads the live session from `sempa_focus_timer` (written by
 *    FocusTimerService) and shows phase, task, minutes-left, and Pause/Resume + Done
 *    buttons that fire back into the service (which relays to the web store and
 *    refreshes this widget).
 *  • IDLE — reads today's not-done tasks from the `sempa_widget` cache (the
 *    WidgetBridge) and offers a task picker; tapping a task opens the app with a
 *    `startFocusTaskId` extra so the web store performs the actual start (carrying
 *    prior-time/estimate and the end-of-session time-log confirm).
 *
 * Glance can't tick per-second, so the time shows coarse minutes-left (refreshed on
 * every state change); the ongoing notification remains the precise, ticking surface.
 */
class SempaPomodoroWidget : GlanceAppWidget() {

    override suspend fun provideGlance(context: Context, id: GlanceId) {
        val fp = context.getSharedPreferences(FocusTimerService.PREFS, Context.MODE_PRIVATE)
        val active = fp.getBoolean(FocusTimerService.KEY_ACTIVE, false)
        val title = fp.getString(FocusTimerService.KEY_TITLE, "Focus") ?: "Focus"
        val phase = fp.getString(FocusTimerService.KEY_PHASE, "Focus") ?: "Focus"
        val running = fp.getBoolean(FocusTimerService.KEY_RUNNING, false)
        val endTime = fp.getLong(FocusTimerService.KEY_END, 0L)
        val remain = fp.getLong(FocusTimerService.KEY_REMAIN, 0L)
        val now = System.currentTimeMillis()
        val remainingMs = if (running && endTime > now) endTime - now else remain
        val minsLeft = if (remainingMs > 0) ceil(remainingMs / 60000.0).toInt() else 0

        // Idle task picker: today's not-done tasks from the WidgetBridge cache.
        val tasks = mutableListOf<Pair<String, String>>() // id to title
        if (!active) {
            val wp = context.getSharedPreferences("sempa_widget", Context.MODE_PRIVATE)
            val count = wp.getInt("task_count", 0)
            for (i in 0 until minOf(count, 10)) {
                val tid = wp.getString("task_${i}_id", null) ?: continue
                if (wp.getBoolean("task_${i}_done", false)) continue
                val ttitle = wp.getString("task_${i}_title", "") ?: ""
                tasks.add(tid to ttitle)
                if (tasks.size >= 4) break
            }
        }

        // A native Chronometer that ticks down on its own (no per-second widget
        // refresh, no battery cost) — the live mirror of the app timer. Its base is
        // in elapsedRealtime() terms; convert from the wall-clock remaining. Only
        // while running (paused shows static text) and on API 24+ (count-down).
        val isFocus = phase.startsWith("Focus", true)
        val chronoRv: RemoteViews? =
            if (active && running && remainingMs > 0 && Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
                RemoteViews(context.packageName, R.layout.widget_focus_chrono).apply {
                    setChronometerCountDown(R.id.focus_chrono, true)
                    setChronometer(R.id.focus_chrono, SystemClock.elapsedRealtime() + remainingMs, null, true)
                    setTextColor(R.id.focus_chrono, if (isFocus) 0xFFB3592E.toInt() else 0xFF22C55E.toInt())
                }
            } else null

        provideContent {
            if (active) {
                ActiveContent(title = title, phase = phase, running = running, minsLeft = minsLeft, chronoRv = chronoRv)
            } else {
                IdleContent(tasks = tasks)
            }
        }
    }
}

@Composable
private fun ActiveContent(title: String, phase: String, running: Boolean, minsLeft: Int, chronoRv: RemoteViews?) {
    val accent = if (phase.startsWith("Focus", true)) SempaWidgetTheme.primary else SempaWidgetTheme.green
    Box(
        modifier = GlanceModifier
            .fillMaxSize()
            .cornerRadius(20.dp)
            .background(SempaWidgetTheme.surface)
            .padding(16.dp),
    ) {
        Column(modifier = GlanceModifier.fillMaxSize()) {
            Text(
                text = phase.uppercase(),
                style = TextStyle(color = ColorProvider(accent), fontSize = 11.sp, fontWeight = FontWeight.Bold),
                maxLines = 1,
            )
            Spacer(modifier = GlanceModifier.height(2.dp))
            Text(
                text = title,
                style = TextStyle(color = ColorProvider(SempaWidgetTheme.onSurface), fontSize = 14.sp, fontWeight = FontWeight.Medium),
                maxLines = 1,
            )
            Spacer(modifier = GlanceModifier.height(6.dp))
            if (chronoRv != null) {
                // Live, self-ticking countdown — mirrors the app timer.
                AndroidRemoteViews(remoteViews = chronoRv)
            } else {
                Text(
                    text = if (!running) "Paused" else if (minsLeft > 0) "$minsLeft min left" else "Almost done",
                    style = TextStyle(color = ColorProvider(SempaWidgetTheme.onSurface), fontSize = 26.sp, fontWeight = FontWeight.Bold),
                    maxLines = 1,
                )
            }

            Spacer(modifier = GlanceModifier.defaultWeight())

            Row(modifier = GlanceModifier.fillMaxWidth(), verticalAlignment = Alignment.Vertical.CenterVertically) {
                // Pause / Resume (primary)
                Box(
                    modifier = GlanceModifier
                        .defaultWeight()
                        .cornerRadius(10.dp)
                        .background(accent)
                        .padding(vertical = 8.dp)
                        .clickable(
                            actionRunCallback<FocusControlAction>(
                                actionParametersOf(FocusControlAction.actionKey to if (running) "pause" else "resume"),
                            ),
                        ),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        text = if (running) "Pause" else "Resume",
                        style = TextStyle(color = ColorProvider(SempaWidgetTheme.onPrimary), fontSize = 13.sp, fontWeight = FontWeight.Medium),
                    )
                }
                Spacer(modifier = GlanceModifier.width(8.dp))
                // Done
                Box(
                    modifier = GlanceModifier
                        .cornerRadius(10.dp)
                        .background(SempaWidgetTheme.accentBg)
                        .padding(horizontal = 14.dp, vertical = 8.dp)
                        .clickable(
                            actionRunCallback<FocusControlAction>(
                                actionParametersOf(FocusControlAction.actionKey to "done"),
                            ),
                        ),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        text = "Done",
                        style = TextStyle(color = ColorProvider(SempaWidgetTheme.primary), fontSize = 13.sp, fontWeight = FontWeight.Medium),
                    )
                }
            }
        }
    }
}

@Composable
private fun IdleContent(tasks: List<Pair<String, String>>) {
    Box(
        modifier = GlanceModifier
            .fillMaxSize()
            .cornerRadius(20.dp)
            .background(SempaWidgetTheme.surface)
            .padding(16.dp),
    ) {
        Column(modifier = GlanceModifier.fillMaxSize()) {
            Row(verticalAlignment = Alignment.Vertical.CenterVertically) {
                Text(
                    text = "▶",
                    style = TextStyle(color = ColorProvider(SempaWidgetTheme.primary), fontSize = 13.sp, fontWeight = FontWeight.Bold),
                )
                Spacer(modifier = GlanceModifier.width(6.dp))
                Text(
                    text = "Focus on…",
                    style = TextStyle(color = ColorProvider(SempaWidgetTheme.onSurface), fontSize = 13.sp, fontWeight = FontWeight.Bold),
                )
            }
            Spacer(modifier = GlanceModifier.height(8.dp))

            if (tasks.isEmpty()) {
                Box(
                    modifier = GlanceModifier
                        .fillMaxWidth()
                        .clickable(actionStartActivity<MainActivity>()),
                ) {
                    Text(
                        text = "Open Sempa to plan today",
                        style = TextStyle(color = ColorProvider(SempaWidgetTheme.onSurfaceDim), fontSize = 13.sp),
                    )
                }
            } else {
                tasks.forEach { (id, title) ->
                    Row(
                        modifier = GlanceModifier
                            .fillMaxWidth()
                            .padding(vertical = 3.dp)
                            .cornerRadius(10.dp)
                            .background(SempaWidgetTheme.accentBg)
                            .padding(horizontal = 10.dp, vertical = 7.dp)
                            .clickable(
                                actionStartActivity<MainActivity>(
                                    actionParametersOf(startFocusTaskIdKey to id),
                                ),
                            ),
                        verticalAlignment = Alignment.Vertical.CenterVertically,
                    ) {
                        Text(
                            text = "▶",
                            style = TextStyle(color = ColorProvider(SempaWidgetTheme.primary), fontSize = 11.sp),
                        )
                        Spacer(modifier = GlanceModifier.width(8.dp))
                        Text(
                            text = title,
                            style = TextStyle(color = ColorProvider(SempaWidgetTheme.onSurface), fontSize = 13.sp),
                            maxLines = 1,
                        )
                    }
                }
            }
        }
    }
}

/** Intent-extra key carried when the user taps a task in the idle widget. */
val startFocusTaskIdKey = ActionParameters.Key<String>("startFocusTaskId")

/**
 * Glance callback for the Pause/Resume/Done buttons: forwards the action to
 * FocusTimerService, which updates state, relays to the web store, and refreshes the
 * widget. Sending a command to the already-running foreground service is permitted
 * even from the widget's background context (the buttons only show when a session is
 * active, i.e. the service is running).
 */
class FocusControlAction : ActionCallback {
    override suspend fun onAction(context: Context, glanceId: GlanceId, parameters: ActionParameters) {
        val svc = when (parameters[actionKey]) {
            "pause" -> FocusTimerService.ACTION_BTN_PAUSE
            "resume" -> FocusTimerService.ACTION_BTN_RESUME
            "done" -> FocusTimerService.ACTION_BTN_DONE
            else -> return
        }
        val i = Intent(context, FocusTimerService::class.java).setAction(svc)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) context.startForegroundService(i)
        else context.startService(i)
    }

    companion object {
        val actionKey = ActionParameters.Key<String>("focus_action")
    }
}

class SempaPomodoroWidgetReceiver : GlanceAppWidgetReceiver() {
    override val glanceAppWidget: GlanceAppWidget = SempaPomodoroWidget()
}
