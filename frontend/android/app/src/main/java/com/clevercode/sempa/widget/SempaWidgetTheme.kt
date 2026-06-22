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
        val onPrimary: Color,
        val surface: Color,
        val surfaceVariant: Color,
        val onSurface: Color,
        val onSurfaceDim: Color,
        val outline: Color,
        val accentBg: Color,
        val green: Color,
    )

    private val DEFAULT = Palette(
        primary = Color(0xFFB3592E),
        onPrimary = Color(0xFFF7F3EB),
        surface = Color(0xFFFAF8F4),
        surfaceVariant = Color(0xFFF0ECE3),
        onSurface = Color(0xFF2B221B),
        onSurfaceDim = Color(0xFFB0A498),
        outline = Color(0xFFE2DCD3),
        accentBg = Color(0xFFEDE5DD),
        green = Color(0xFF22C55E),
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
        palette = Palette(
            primary = col("theme_primary", DEFAULT.primary),
            onPrimary = DEFAULT.onPrimary, // on-accent text stays light at both ends
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
    val onPrimary get() = palette.onPrimary
    val surface get() = palette.surface
    val surfaceVariant get() = palette.surfaceVariant
    val onSurface get() = palette.onSurface
    val onSurfaceDim get() = palette.onSurfaceDim
    val outline get() = palette.outline
    val accentBg get() = palette.accentBg
    val green get() = palette.green
}
