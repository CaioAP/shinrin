<script setup lang="ts">
// Client-only (.client.vue): lightweight-charts draws on a canvas.
import { AreaSeries, HistogramSeries, createChart } from 'lightweight-charts'
import type { IChartApi, ISeriesApi } from 'lightweight-charts'

const props = defineProps<{ bars: PriceBar[], currency: string }>()
const { locale } = useI18n()
const colorMode = useColorMode()

const el = useTemplateRef<HTMLDivElement>('el')
let chart: IChartApi | undefined
let price: ISeriesApi<'Area'> | undefined
let volume: ISeriesApi<'Histogram'> | undefined

function theme() {
  const dark = colorMode.value === 'dark'
  return {
    layout: { background: { color: 'transparent' }, textColor: dark ? '#a1a1aa' : '#52525b', attributionLogo: false },
    grid: { vertLines: { color: dark ? '#27272a' : '#f4f4f5' }, horzLines: { color: dark ? '#27272a' : '#f4f4f5' } },
  }
}

function render() {
  if (!price || !volume || !chart) return
  price.setData(props.bars.map(b => ({ time: b.date, value: b.adjClose || b.close })))
  volume.setData(props.bars.map(b => ({ time: b.date, value: b.volume })))
  chart.timeScale().fitContent()
}

onMounted(() => {
  if (!el.value) return
  chart = createChart(el.value, {
    ...theme(),
    autoSize: true,
    rightPriceScale: { borderVisible: false },
    timeScale: { borderVisible: false },
    localization: { locale: locale.value },
  })
  price = chart.addSeries(AreaSeries, {
    lineColor: '#10b981',
    topColor: 'rgba(16, 185, 129, 0.3)',
    bottomColor: 'rgba(16, 185, 129, 0)',
    lineWidth: 2,
    priceFormat: { type: 'custom', formatter: (v: number) => formatMoney(v, props.currency, locale.value) },
  })
  volume = chart.addSeries(HistogramSeries, { color: 'rgba(113, 113, 122, 0.35)', priceScaleId: '', priceFormat: { type: 'volume' }, lastValueVisible: false, priceLineVisible: false })
  volume.priceScale().applyOptions({ scaleMargins: { top: 0.8, bottom: 0 } })
  render()
})

watch(() => props.bars, render)
watch(() => colorMode.value, () => chart?.applyOptions(theme()))
onBeforeUnmount(() => chart?.remove())
</script>

<template>
  <div ref="el" class="h-72 w-full" />
</template>
