package com.clevercode.sempa

import android.content.Context
import android.content.Intent
import android.os.Build
import com.getcapacitor.JSObject
import com.getcapacitor.Plugin
import com.getcapacitor.PluginCall
import com.getcapacitor.PluginMethod
import com.getcapacitor.annotation.CapacitorPlugin

/**
 * Bridge between the web Pomodoro timer and the native FocusTimerService.
 *
 * `show` starts/updates the ongoing countdown notification; `stop` clears it.
 * Pause/Done taps from the notification are relayed back to JS via the
 * `focusAction` event (live) and, when the app was closed at tap time, drained on
 * next launch via `consumePendingAction`.
 */
@CapacitorPlugin(name = "FocusTimer")
class FocusTimerPlugin : Plugin() {

    override fun load() {
        active = this
    }

    override fun handleOnDestroy() {
        if (active === this) active = null
        super.handleOnDestroy()
    }

    @PluginMethod
    fun show(call: PluginCall) {
        val ctx = context ?: return call.reject("no context")
        val i = Intent(ctx, FocusTimerService::class.java).apply {
            action = FocusTimerService.ACTION_SHOW
            putExtra(FocusTimerService.EXTRA_TITLE, call.getString("title") ?: "Focus")
            putExtra(FocusTimerService.EXTRA_PHASE, call.getString("phase") ?: "Focus")
            putExtra(FocusTimerService.EXTRA_RUNNING, call.getBoolean("running", true) ?: true)
            // Epoch millis as Double (exact below 2^53) — PluginCall has no getLong.
            putExtra(FocusTimerService.EXTRA_END, (call.getDouble("endTime") ?: 0.0).toLong())
            putExtra(FocusTimerService.EXTRA_REMAINING, (call.getDouble("remaining") ?: 0.0).toLong())
        }
        startService(ctx, i)
        call.resolve()
    }

    @PluginMethod
    fun stop(call: PluginCall) {
        val ctx = context ?: return call.reject("no context")
        // Clear any stale pending action so it can't fire on next launch.
        ctx.getSharedPreferences(FocusTimerService.PREFS, Context.MODE_PRIVATE)
            .edit().remove(FocusTimerService.KEY_PENDING).apply()
        startService(ctx, Intent(ctx, FocusTimerService::class.java).setAction(FocusTimerService.ACTION_STOP))
        call.resolve()
    }

    /** Returns and clears any action a notification button fired while JS was asleep. */
    @PluginMethod
    fun consumePendingAction(call: PluginCall) {
        val ctx = context ?: return call.reject("no context")
        val prefs = ctx.getSharedPreferences(FocusTimerService.PREFS, Context.MODE_PRIVATE)
        val action = prefs.getString(FocusTimerService.KEY_PENDING, null)
        if (action != null) prefs.edit().remove(FocusTimerService.KEY_PENDING).apply()
        call.resolve(JSObject().put("action", action ?: ""))
    }

    /** Returns and clears a task id stashed by the home-screen widget's "start this
     *  task" tap, so the web store performs the actual start (prior time, estimate,
     *  end-of-session confirm). */
    @PluginMethod
    fun consumePendingStart(call: PluginCall) {
        val ctx = context ?: return call.reject("no context")
        val prefs = ctx.getSharedPreferences(FocusTimerService.PREFS, Context.MODE_PRIVATE)
        val taskId = prefs.getString(FocusTimerService.KEY_PENDING_START, null)
        if (taskId != null) prefs.edit().remove(FocusTimerService.KEY_PENDING_START).apply()
        call.resolve(JSObject().put("taskId", taskId ?: ""))
    }

    private fun startService(ctx: Context, i: Intent) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) ctx.startForegroundService(i)
        else ctx.startService(i)
    }

    companion object {
        @Volatile private var active: FocusTimerPlugin? = null

        /** Deliver a notification-button action to JS live; clears the prefs stash
         *  so it isn't re-applied by consumePendingAction. No-op if app is asleep. */
        fun emit(ctx: Context, action: String) {
            val p = active ?: return
            ctx.getSharedPreferences(FocusTimerService.PREFS, Context.MODE_PRIVATE)
                .edit().remove(FocusTimerService.KEY_PENDING).apply()
            p.notifyListeners("focusAction", JSObject().put("action", action))
        }

        /** Deliver a widget "start this task" tap to JS live (app already running);
         *  clears the prefs stash so consumePendingStart doesn't re-fire it. */
        fun emitStart(ctx: Context, taskId: String) {
            val p = active ?: return
            ctx.getSharedPreferences(FocusTimerService.PREFS, Context.MODE_PRIVATE)
                .edit().remove(FocusTimerService.KEY_PENDING_START).apply()
            p.notifyListeners("focusStart", JSObject().put("taskId", taskId))
        }
    }
}
