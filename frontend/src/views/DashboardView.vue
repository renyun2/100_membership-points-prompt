<script setup>
import { onMounted, ref, watch, nextTick } from 'vue'
import * as echarts from 'echarts'
import { api } from '../api/client'
import dayjs from 'dayjs'

const loading = ref(false)
const active = ref(null)
const trend = ref([])
const tiers = ref([])
const topMembers = ref([])

let chartMonthly = null
let chartPie = null
const monthlyRef = ref(null)
const tierRef = ref(null)

async function load() {
  loading.value = true
  try {
    const [a, tr, tier, tops] = await Promise.all([
      api.get('/stats/active-members'),
      api.get('/stats/monthly-trend'),
      api.get('/stats/tier-distribution'),
      api.get('/stats/top-balances'),
    ])
    active.value = a.data?.active_members_last_30d ?? 0
    trend.value = tr.data ?? []
    tiers.value = tier.data ?? []
    topMembers.value = tops.data?.items ?? tops.data ?? []

    await nextTick()
    renderCharts()
  } finally {
    loading.value = false
  }
}

function renderCharts() {
  const elBar = monthlyRef.value
  if (elBar) {
    if (!chartMonthly) chartMonthly = echarts.init(elBar)
    chartMonthly.setOption({
      tooltip: { trigger: 'axis' },
      legend: {},
      grid: { left: 44, right: 16, bottom: 40, containLabel: true },
      xAxis: {
        type: 'category',
        data: trend.value.map((r) =>
          typeof r.month === 'string'
            ? r.month.slice(0, 10)
            : dayjs(r.month).format('YYYY-MM'),
        ),
      },
      yAxis: { type: 'value', name: '积分' },
      series: [
        { name: '发放', type: 'line', smooth: true, data: trend.value.map((r) => r.issued ?? 0) },
        {
          name: '消耗(兑换/抵扣)',
          type: 'bar',
          data: trend.value.map((r) => r.redeemed ?? 0),
        },
        {
          name: '过期核销',
          type: 'bar',
          stack: 'x',
          data: trend.value.map((r) => r.expired_adjusted ?? 0),
        },
      ],
    })
  }
  const elPie = tierRef.value
  if (elPie) {
    if (!chartPie) chartPie = echarts.init(elPie)
    chartPie.setOption({
      tooltip: { trigger: 'item' },
      legend: { bottom: 0 },
      series: [
        {
          type: 'pie',
          radius: ['32%', '64%'],
          data: tiers.value.map((t) => ({ name: t.tier_name || t.tier_code, value: t.n })),
          label: { formatter: '{b}:{c}' },
        },
      ],
    })
  }
}

watch(trend, () => nextTick().then(renderCharts))

onMounted(() => {
  load().then(() => nextTick()).then(renderCharts)
  window.addEventListener(
    'resize',
    () => {
      chartMonthly?.resize()
      chartPie?.resize()
    },
    { passive: true },
  )
})
</script>

<template>
  <div v-loading="loading" class="page">
    <el-row :gutter="16">
      <el-col :span="24">
        <el-card shadow="never">
          <div class="subtitle">活跃会员（最近 30 天有积分增减）</div>
          <div class="number">{{ active ?? '—' }}</div>
          <div class="hint">
            同时展示月度发放/消耗/过期核销趋势与各等级占比，数据来源预置演示流水。
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="16" class="mt">
      <el-col :xs="24" :md="14">
        <el-card shadow="never" header="月度趋势">
          <div ref="monthlyRef" class="chart monthly" />
        </el-card>
      </el-col>
      <el-col :xs="24" :md="10">
        <el-card shadow="never" header="等级分布">
          <div ref="tierRef" class="chart pie" />
        </el-card>
      </el-col>
    </el-row>

    <el-card class="mt" shadow="never" header="高价值会员（积分余额 TOP100）">
      <el-table :data="topMembers" size="small" height="420">
        <el-table-column prop="phone" label="手机号" width="140" />
        <el-table-column prop="name" label="姓名" width="140" />
        <el-table-column prop="tier_name" label="等级" width="96" />
        <el-table-column prop="lifetime_points" label="累计入账积分" />
        <el-table-column prop="points_balance" label="当前余额" />
        <el-table-column prop="tier_code" label="等级编码" width="140" />
      </el-table>
    </el-card>
  </div>
</template>

<style scoped>
.page {
  max-width: 1200px;
}
.subtitle {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.number {
  font-size: 32px;
  font-weight: 600;
}
.hint {
  margin-top: 8px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.chart {
  width: 100%;
}
.chart.monthly {
  height: 360px;
}
.chart.pie {
  height: 360px;
}
.mt {
  margin-top: 16px;
}
</style>
