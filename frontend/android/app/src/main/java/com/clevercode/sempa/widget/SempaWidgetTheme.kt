package com.clevercode.sempa.widget

import android.content.Context
import androidx.compose.ui.graphics.Color

/**
 * Widget colour palette. The web app pushes the ACTIVE theme's resolved colours to
 * the `sempa_widget` prefs (theme_* keys) via WidgetBridge; call [refresh] before
 * rendering and the getters return those colours, so every widget tracks the app's
 * theme (accent preset + light/dark). Falls back to the terracotta brand defaults
 * until the app has pushed a theme.
 */
object SempaWidgetTheme {

    data class Palette(
        val primary: Color,
        /** Solid fill behind text (the app's button colour) and its text colour. */
        val primaryFill: Color,
        val onPrimary: Color,
        val surface: Color,
        val surfaceVariant: Color,
        val onSurface: Color,
        val onSurfaceDim: Color,
        val outline: Color,
        val accentBg: Color,
        val green: Color,
    )

    // Terracotta light, 1.27 tokens (frontend/src/themes.css): surface is the
    // raised card (--sempa-bg-panel), surfaceVariant the inset fill on it.
    private val DEFAULT = Palette(
        primary = Color(0xFFB3592E),
        primaryFill = Color(0xFFB3592E),
        onPrimary = Color(0xFFFFFAF5),
        surface = Color(0xFFFFFFFF),
        surfaceVariant = Color(0xFFF6F0E8),
        onSurface = Color(0xFF2B221B),
        onSurfaceDim = Color(0xFF8A7B6E),
        outline = Color(0xFFE6DDD1),
        accentBg = Color(0xFFF6E5D9),
        green = Color(0xFF4A8656),
    )

    @Volatile
    private var palette = DEFAULT

    /** Reload the palette from the colours the web app pushed (best-effort). */
    fun refresh(context: Context) {
        val p = context.getSharedPreferences("sempa_widget", Context.MODE_PRIVATE)
        fun col(key: String, fallback: Color): Color {
            val s = p.getString(key, null) ?: return fallback
            return try {
                Color(android.graphics.Color.parseColor(s))
            } catch (_: Exception) {
                fallback
            }
        }
        val accent = col("theme_primary", DEFAULT.primary)
        palette = Palette(
            primary = accent,
            // Text on a solid accent block uses the app's button pair (1.28), not
            // the accent: in dark themes the accent is lightened for text on dark
            // surfaces, so light text on it failed contrast (2.9:1 in terracotta),
            // and the dark Forest/Plum/Slate/Ocean/OLED buttons use dark text.
            // Falls back to the accent + light text if the app hasn't pushed it.
            primaryFill = col("theme_primary_fill", accent),
            onPrimary = col("theme_on_primary_fill", DEFAULT.onPrimary),
            surface = col("theme_surface", DEFAULT.surface),
            surfaceVariant = col("theme_surface", DEFAULT.surfaceVariant),
            onSurface = col("theme_on_surface", DEFAULT.onSurface),
            onSurfaceDim = col("theme_on_surface_dim", DEFAULT.onSurfaceDim),
            outline = col("theme_outline", DEFAULT.outline),
            accentBg = col("theme_accent_bg", DEFAULT.accentBg),
            green = col("theme_green", DEFAULT.green),
        )
    }

    val primary get() = palette.primary
    val primaryFill get() = palette.primaryFill
    val onPrimary get() = palette.onPrimary
    val surface get() = palette.surface
    val surfaceVariant get() = palette.surfaceVariant
    val onSurface get() = palette.onSurface
    val onSurfaceDim get() = palette.onSurfaceDim
    val outline get() = palette.outline
    val accentBg get() = palette.accentBg
    val green get() = palette.green
}
