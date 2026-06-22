package com.clevercode.sempa

import android.content.Context
import androidx.glance.appwidget.updateAll
import com.clevercode.sempa.widget.SempaFocusWidget
import com.clevercode.sempa.widget.SempaLargeWidget
import com.clevercode.sempa.widget.SempaMediumWidget
import com.clevercode.sempa.widget.SempaSmallWidget
import com.getcapacitor.JSObject
import com.getcapacitor.Plugin
import com.getcapacitor.PluginCall
import com.getcapacitor.PluginMethod
import com.getcapacitor.annotation.CapacitorPlugin
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch

@CapacitorPlugin(name = "WidgetBridge")
class WidgetBridgePlugin : Plugin() {

    @PluginMethod
    fun updateWidgetData(call: PluginCall) {
        val ctx = context ?: run {
            call.reject("No context")
            return
        }

        val prefs = ctx.getSharedPreferences("sempa_widget", Context.MODE_PRIVATE)
        val editor = prefs.edit()

        // Today stats
        call.getInt("todayTotal")?.let { editor.putInt("today_total", it) }
        call.getInt("todayDone")?.let { editor.putInt("today_done", it) }

        // Task list (for medium/large widgets)
        val tasks = call.getArray("tasks")
        if (tasks != null) {
            // Clear old task entries
            for (i in 0..9) {
                editor.remove("task_${i}_title")
                editor.remove("task_${i}_done")
                editor.remove("task_${i}_id")
            }
            for (i in 0 until minOf(tasks.length(), 10)) {
                val t = tasks.getJSONObject(i)
                editor.putString("task_${i}_title", t.optString("title", ""))
                editor.putBoolean("task_${i}_done", t.optBoolean("done", false))
                // Task id powers the Pomodoro widget's "start this task" tap.
                editor.putString("task_${i}_id", t.optString("id", ""))
            }
            editor.putInt("task_count", minOf(tasks.length(), 10))
        }

        // Week data (for large widget)
        val week = call.getArray("week")
        if (week != null) {
            for (i in 0 until minOf(week.length(), 7)) {
                val d = week.getJSONObject(i)
                val date = d.optString("date", "")
                val count = d.optInt("count", 0)
                if (date.isNotEmpty()) {
                    editor.putInt("week_${date}_count", count)
                }
            }
        }

        // Active theme colours (pushed by the web app) so every widget matches the
        // app's theme — accent preset and light/dark. Only overwrite when present,
        // so a task-only push doesn't wipe the theme and vice-versa.
        val theme = call.getObject("theme")
        if (theme != null) {
            for (k in arrayOf("primary", "accent_bg", "surface", "on_surface", "on_surface_dim", "outline", "green")) {
                val v = theme.optString(k, "")
                if (v.isNotEmpty()) editor.putString("theme_$k", v)
            }
        }

        editor.apply()

        // Trigger widget refresh
        CoroutineScope(Dispatchers.IO).launch {
            try {
                SempaSmallWidget().updateAll(ctx)
                SempaMediumWidget().updateAll(ctx)
                SempaLargeWidget().updateAll(ctx)
                SempaFocusWidget.updateAll(ctx)
            } catch (_: Exception) {}
        }

        call.resolve()
    }
}
