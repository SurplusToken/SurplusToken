/**
 * Helicone 图表色板(来源 helicone/web/lib/chartColors.ts)
 * 全站图表统一从这里取色,不要散落硬编码 hex。
 */

export const chartColors = {
  success: 'hsl(145, 80%, 42%)',
  error: 'hsl(0, 100%, 50%)',
  blue: 'hsl(217, 100%, 55%)',
  purple: 'hsl(271, 100%, 60%)',
  orange: 'hsl(25, 100%, 50%)'
} as const

/** 补充语义色(缓存/次要系列用),与上面五色协调 */
export const chartSupplementaryColors = {
  cyan: 'hsl(190, 90%, 42%)',
  pink: 'hsl(330, 80%, 55%)',
  amber: 'hsl(45, 95%, 45%)',
  indigo: 'hsl(245, 70%, 62%)',
  teal: 'hsl(170, 75%, 36%)'
} as const

/** 按序取色的 10 色调色板:前 5 个为 Helicone 五色,后 5 个为补充协调色 */
export const chartPalette: string[] = [
  chartColors.success,
  chartColors.error,
  chartColors.blue,
  chartColors.purple,
  chartColors.orange,
  chartSupplementaryColors.cyan,
  chartSupplementaryColors.pink,
  chartSupplementaryColors.amber,
  chartSupplementaryColors.indigo,
  chartSupplementaryColors.teal
]

/** 图表网格线/刻度文字色,随明暗主题切换(对齐 CSS 变量 token) */
export const chartThemeColors = {
  light: {
    grid: 'hsl(214, 32%, 91%)',
    text: 'hsl(215, 16%, 47%)'
  },
  dark: {
    grid: 'hsl(217, 33%, 17%)',
    text: 'hsl(215, 20%, 65%)'
  }
} as const

export const getChartThemeColors = (isDark: boolean) =>
  isDark ? chartThemeColors.dark : chartThemeColors.light

/** 给 hsl() 色值加透明度,返回 hsla();用于面积图填充 */
export const withAlpha = (color: string, alpha: number): string => {
  const match = color.match(/^hsl\(([^)]+)\)$/)
  if (match) return `hsla(${match[1]}, ${alpha})`
  return color
}
