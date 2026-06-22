package com.clevercode.sempa

import android.content.ComponentName
import android.content.Context
import android.content.pm.PackageManager
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

    /**
     * Switch the launcher icon to the variant matching the given theme by enabling
     * that activity-alias and disabling the others. The web app calls this only when
     * the user opts in (after a theme change), so the brief launcher refresh is
     * expected. Enables the target FIRST (a launcher entry always exists) and no-ops
     * when it's already current (no needless flicker).
     */
    @PluginMethod
    fun setAppIcon(call: PluginCall) {
        val ctx = context ?: return call.reject("No context")
        val theme = call.getString("theme") ?: return call.reject("theme required")
        val target = ALIASES[theme] ?: return call.resolve() // unknown theme → leave as-is
        val pm = ctx.packageManager
        val pkg = ctx.packageName
        fun comp(alias: String) = ComponentName(pkg, "$pkg.$alias")

        val explicit = ALIASES.values.firstOrNull {
            pm.getComponentEnabledSetting(comp(it)) == PackageManager.COMPONENT_ENABLED_STATE_ENABLED
        }
        val current = explicit ?: ALIASES.getValue("terracotta") // manifest default
        if (current == target) return call.resolve()

        pm.setComponentEnabledSetting(comp(target), PackageManager.COMPONENT_ENABLED_STATE_ENABLED, PackageManager.DONT_KILL_APP)
        for (alias in ALIASES.values) {
            if (alias != target) {
                pm.setComponentEnabledSetting(comp(alias), PackageManager.COMPONENT_ENABLED_STATE_DISABLED, PackageManager.DONT_KILL_APP)
            }
        }
        call.resolve()
    }

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

    companion object {
        // theme name (from the web theme store) → launcher activity-alias.
        private val ALIASES = mapOf(
            "terracotta" to "IconTerracotta",
            "forest" to "IconForest",
            "plum" to "IconPlum",
            "slate" to "IconSlate",
            "oled" to "IconOled",
            "ocean" to "IconOcean",
        )
    }
}
