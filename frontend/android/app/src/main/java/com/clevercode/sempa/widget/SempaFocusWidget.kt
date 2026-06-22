package com.clevercode.sempa.widget

import android.app.PendingIntent
import android.appwidget.AppWidgetManager
import android.appwidget.AppWidgetProvider
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.res.ColorStateList
import android.os.Build
import android.os.SystemClock
import android.view.View
import android.widget.RemoteViews
import androidx.compose.ui.graphics.toArgb
import com.clevercode.sempa.FocusTimerService
import com.clevercode.sempa.MainActivity
import com.clevercode.sempa.R
import kotlin.math.ceil

/**
 * Home-screen Focus widget — a CLASSIC RemoteViews AppWidgetProvider (not Glance) so
 * we get a real, self-ticking native Chronometer and full layout control at any size.
 * Three states (from SharedPreferences, no JS needed while closed):
 *
 *  • ACTIVE — live timer (phase, task, ticking countdown, Pause/Resume + Done).
 *  • TASKS  — idle with today's not-done tasks to start.
 *  • REST   — idle with no tasks: a calm resting face, so it looks good on the home
 *    screen all the time.
 *
 * Colours track the app's active theme via [SempaWidgetTheme] (pushed by WidgetBridge).
 */
class SempaFocusWidget : AppWidgetProvider() {

    override fun onUpdate(context: Context, manager: AppWidgetManager, ids: IntArray) {
        val rv = buildViews(context)
        for (id in ids) manager.updateAppWidget(id, rv)
    }

    companion object {
        private val ROW_IDS = intArrayOf(
            R.id.focus_task_0, R.id.focus_task_1, R.id.focus_task_2, R.id.focus_task_3,
        )

        /** Re-render every placed Focus widget. Safe to call from any thread. */
        fun updateAll(context: Context) {
            val manager = AppWidgetManager.getInstance(context) ?: return
            val ids = manager.getAppWidgetIds(ComponentName(context, SempaFocusWidget::class.java))
            if (ids == null || ids.isEmpty()) return
            val rv = buildViews(context)
            for (id in ids) manager.updateAppWidget(id, rv)
        }

        private fun buildViews(context: Context): RemoteViews {
            SempaWidgetTheme.refresh(context)
            val rv = RemoteViews(context.packageName, R.layout.widget_focus)

            // Theme the card + pill backgrounds (tinting keeps the rounded shape).
            // setColorStateList (background tint) is API 31+; on older the layout's
            // brand defaults remain. Text/accent colours below theme on all versions.
            tintBg(rv, R.id.focus_root, SempaWidgetTheme.surface.toArgb())
            for (id in intArrayOf(R.id.focus_toggle, R.id.focus_done, *ROW_IDS)) {
                tintBg(rv, id, SempaWidgetTheme.accentBg.toArgb())
            }

            val fp = context.getSharedPreferences(FocusTimerService.PREFS, Context.MODE_PRIVATE)
            val active = fp.getBoolean(FocusTimerService.KEY_ACTIVE, false)

            if (active) {
                rv.setViewVisibility(R.id.focus_active, View.VISIBLE)
                rv.setViewVisibility(R.id.focus_tasks, View.GONE)
                rv.setViewVisibility(R.id.focus_rest, View.GONE)
                renderActive(context, rv, fp)
            } else {
                rv.setViewVisibility(R.id.focus_active, View.GONE)
                renderIdle(context, rv)
            }
            return rv
        }

        private fun renderActive(context: Context, rv: RemoteViews, fp: android.content.SharedPreferences) {
            val title = fp.getString(FocusTimerService.KEY_TITLE, "Focus") ?: "Focus"
            val phase = fp.getString(FocusTimerService.KEY_PHASE, "Focus") ?: "Focus"
            val running = fp.getBoolean(FocusTimerService.KEY_RUNNING, false)
            val endTime = fp.getLong(FocusTimerService.KEY_END, 0L)
            val remain = fp.getLong(FocusTimerService.KEY_REMAIN, 0L)
            val now = System.currentTimeMillis()
            val remainingMs = if (running && endTime > now) endTime - now else remain
            val accent = if (phase.startsWith("Focus", true)) SempaWidgetTheme.primary.toArgb() else SempaWidgetTheme.green.toArgb()
            val onSurface = SempaWidgetTheme.onSurface.toArgb()

            rv.setTextViewText(R.id.focus_phase, phase.uppercase())
            rv.setTextColor(R.id.focus_phase, accent)
            rv.setTextViewText(R.id.focus_title, title)
            rv.setTextColor(R.id.focus_title, onSurface)

            if (running && remainingMs > 0 && Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
                // Live, self-ticking countdown. Chronometer base is in elapsedRealtime().
                rv.setChronometerCountDown(R.id.focus_chrono, true)
                rv.setChronometer(R.id.focus_chrono, SystemClock.elapsedRealtime() + remainingMs, null, true)
                rv.setTextColor(R.id.focus_chrono, onSurface)
                rv.setViewVisibility(R.id.focus_chrono, View.VISIBLE)
                rv.setViewVisibility(R.id.focus_static, View.GONE)
            } else {
                rv.setChronometer(R.id.focus_chrono, SystemClock.elapsedRealtime(), null, false)
                rv.setViewVisibility(R.id.focus_chrono, View.GONE)
                rv.setViewVisibility(R.id.focus_static, View.VISIBLE)
                rv.setTextColor(R.id.focus_static, onSurface)
                val mins = if (remainingMs > 0) ceil(remainingMs / 60000.0).toInt() else 0
                rv.setTextViewText(R.id.focus_static, if (!running) "Paused" else if (mins > 0) "$mins min" else "Done soon")
            }

            rv.setTextViewText(R.id.focus_toggle, if (running) "Pause" else "Resume")
            rv.setTextColor(R.id.focus_toggle, SempaWidgetTheme.primary.toArgb())
            rv.setTextColor(R.id.focus_done, SempaWidgetTheme.primary.toArgb())
            rv.setOnClickPendingIntent(
                R.id.focus_toggle,
                servicePI(context, if (running) FocusTimerService.ACTION_BTN_PAUSE else FocusTimerService.ACTION_BTN_RESUME, 11),
            )
            rv.setOnClickPendingIntent(R.id.focus_done, servicePI(context, FocusTimerService.ACTION_BTN_DONE, 12))
        }

        private fun renderIdle(context: Context, rv: RemoteViews) {
            val wp = context.getSharedPreferences("sempa_widget", Context.MODE_PRIVATE)
            val count = wp.getInt("task_count", 0)
            val tasks = ArrayList<Pair<String, String>>()
            var i = 0
            while (i < minOf(count, 10) && tasks.size < ROW_IDS.size) {
                val id = wp.getString("task_${i}_id", null)
                val done = wp.getBoolean("task_${i}_done", false)
                val text = wp.getString("task_${i}_title", "") ?: ""
                if (!id.isNullOrEmpty() && !done) tasks.add(id to text)
                i++
            }

            val onSurface = SempaWidgetTheme.onSurface.toArgb()

            if (tasks.isEmpty()) {
                // REST: calm resting face.
                rv.setViewVisibility(R.id.focus_tasks, View.GONE)
                rv.setViewVisibility(R.id.focus_rest, View.VISIBLE)
                rv.setInt(R.id.focus_rest_face, "setColorFilter", SempaWidgetTheme.primary.toArgb())
                rv.setTextColor(R.id.focus_rest_title, onSurface)
                rv.setTextColor(R.id.focus_rest_sub, SempaWidgetTheme.onSurfaceDim.toArgb())
                rv.setOnClickPendingIntent(R.id.focus_rest, activityPI(context, null, 30))
            } else {
                // TASKS: pick one to start.
                rv.setViewVisibility(R.id.focus_tasks, View.VISIBLE)
                rv.setViewVisibility(R.id.focus_rest, View.GONE)
                rv.setTextColor(R.id.focus_tasks_header, onSurface)
                for (idx in ROW_IDS.indices) {
                    val rowId = ROW_IDS[idx]
                    if (idx < tasks.size) {
                        rv.setTextViewText(rowId, "▶  ${tasks[idx].second}")
                        rv.setTextColor(rowId, onSurface)
                        rv.setViewVisibility(rowId, View.VISIBLE)
                        rv.setOnClickPendingIntent(rowId, activityPI(context, tasks[idx].first, 20 + idx))
                    } else {
                        rv.setViewVisibility(rowId, View.GONE)
                    }
                }
            }
        }

        /** Tint a view's background drawable (keeps its rounded shape). API 31+ only. */
        private fun tintBg(rv: RemoteViews, viewId: Int, color: Int) {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                rv.setColorStateList(viewId, "setBackgroundTintList", ColorStateList.valueOf(color))
            }
        }

        /** PendingIntent that sends a control action to the (foreground) focus service. */
        private fun servicePI(context: Context, action: String, req: Int): PendingIntent {
            val i = Intent(context, FocusTimerService::class.java).setAction(action)
            val flags = PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT
            return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O)
                PendingIntent.getForegroundService(context, req, i, flags)
            else
                PendingIntent.getService(context, req, i, flags)
        }

        /** PendingIntent that opens the app, optionally to start a specific task. */
        private fun activityPI(context: Context, taskId: String?, req: Int): PendingIntent {
            val i = Intent(context, MainActivity::class.java).apply {
                flags = Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP
                if (!taskId.isNullOrEmpty()) putExtra("startFocusTaskId", taskId)
            }
            return PendingIntent.getActivity(
                context, req, i, PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
            )
        }
    }
}
