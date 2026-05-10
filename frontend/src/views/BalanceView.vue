<script setup>
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api/client'

const phone = ref('')
const loading = ref(false)

const summary = ref(null)
const summaryTier = ref('')
const expSoon = ref(0)

const txPages = reactive({ total: 0, page: 1, page_size: 30 })
const txRows = ref([])

async function lookup() {
  if (!phone.value.trim()) return ElMessage.warning('请输入手机号')
  loading.value = true
  try {
    const { data: member } = await api.get(`/members/by-phone/${encodeURIComponent(phone.value.trim())}`)
    summary.value = member
    const { data: sx } = await api.get(`/members/${member.id}/summary`)
    summaryTier.value = sx.tier_name
    expSoon.value = sx.approx_expiring_within_30d ?? 0
    txPages.page = 1
    await loadTx(member.id)
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || '未找到会员或查询失败')
  } finally {
    loading.value = false
  }
}

async function loadTx(mid) {
  loading.value = true
  try {
    const { data } = await api.get(`/members/${mid}/transactions`, {
      params: { page: txPages.page, page_size: txPages.page_size },
    })
    txRows.value = data.items ?? []
    txPages.total = data.total ?? 0
  } finally {
    loading.value = false
  }
}

function nextPage() {
  if (!summary.value) return
  loadTx(summary.value.id)
}
</script>

<template>
  <div v-loading="loading">
    <el-row :gutter="12">
      <el-col :span="14">
        <el-input v-model="phone" clearable placeholder="输入手机号后查询" />
      </el-col>
      <el-col :span="10">
        <el-button type="primary" @click="lookup">查询</el-button>
      </el-col>
    </el-row>

    <el-card v-if="summary" shadow="never" class="mt" header="当前会员概览">
      <div class="grid">
        <div><label>手机号</label> {{ summary.phone }}</div>
        <div><label>姓名</label> {{ summary.name }}</div>
        <div><label>等级</label> {{ summaryTier || summary.tier_code }}</div>
        <div><label>累计入账</label> {{ summary.lifetime_points }}</div>
        <div><label>余额</label> {{ summary.points_balance }}</div>
        <div><label>30 天内将到期入账(估算)</label> {{ expSoon }}</div>
      </div>
    </el-card>

    <el-card v-if="summary" shadow="never" header="积分流水">
      <el-table :data="txRows" height="460" size="small">
        <el-table-column prop="occurred_at" label="发生时间">
          <template #default="{ row }">
            <span>{{ (row.occurred_at || '').replace('T', ' ').slice(0, 19) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="direction" label="方向" width="80" />
        <el-table-column prop="points_delta" label="积分变动" width="120" />
        <el-table-column prop="source_type" label="来源" width="140" />
        <el-table-column prop="reason" label="说明" />
        <el-table-column prop="operator" label="操作人" width="120" />
        <el-table-column prop="expires_at" label="到期日（入账）" width="140">
          <template #default="{ row }">
            <span>{{ row.expires_at ? String(row.expires_at).slice(0, 10) : '—' }}</span>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination
        v-model:current-page="txPages.page"
        v-model:page-size="txPages.page_size"
        class="pager"
        :total="txPages.total"
        layout="total, prev, pager, next"
        background
        @current-change="
          () => {
            nextPage()
          }
        "
      />
    </el-card>
  </div>
</template>

<style scoped>
.mt {
  margin-top: 12px;
}
.grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 8px 16px;
}
label {
  color: var(--el-text-color-secondary);
  margin-right: 8px;
}
.pager {
  margin-top: 12px;
}
</style>
