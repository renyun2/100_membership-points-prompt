<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api/client'

const tiers = ref([])
const loading = ref(false)

const rulesForm = reactive({
  spend_yuan_per_point: 10,
  birthday_multiplier: 2,
  campaign_multiplier: 1,
  campaign_end: '',
  expiry_jan_clear_after_years: 2,
  active_event_name: '',
})

async function load() {
  loading.value = true
  try {
    const [{ data: t }, { data: r }] = await Promise.all([
      api.get('/tier-configs'),
      api.get('/point-rules'),
    ])
    tiers.value = t
    rulesForm.spend_yuan_per_point = r.spend_yuan_per_point
    rulesForm.birthday_multiplier = r.birthday_multiplier
    rulesForm.campaign_multiplier = r.campaign_multiplier
    rulesForm.expiry_jan_clear_after_years = r.expiry_jan_clear_after_years
    rulesForm.active_event_name = r.active_event_name
    rulesForm.campaign_end = r.campaign_end
      ? String(r.campaign_end).slice(0, 10)
      : ''
  } catch (e) {
    ElMessage.error(String(e.message || e))
  } finally {
    loading.value = false
  }
}

async function saveTiers() {
  try {
    const { data } = await api.put('/tier-configs', tiers.value)
    tiers.value = data
    ElMessage.success('已保存等级阈值')
    load()
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || '保存失败')
  }
}

async function saveRules() {
  try {
    const body = { ...rulesForm }
    if (!body.campaign_end) delete body.campaign_end
    await api.put('/point-rules', body)
    ElMessage.success('已保存积分规则')
    load()
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || '保存失败')
  }
}

onMounted(load)
</script>

<template>
  <div v-loading="loading">
    <el-card shadow="never" header="会员等级阈值">
      <p class="muted">
        根据累计入账积分从高到低选取首个满足阈值门槛的等级。修改后会自动校准所有会员的等级字段。
      </p>
      <el-table :data="tiers" size="small" class="mt">
        <el-table-column prop="tier_code" label="编码" width="160" />
        <el-table-column prop="tier_name" label="显示名称">
          <template #default="{ row }">
            <el-input v-model="row.tier_name" />
          </template>
        </el-table-column>
        <el-table-column prop="min_lifetime_points" label="最少累计入账积分">
          <template #default="{ row }">
            <el-input v-model.number="row.min_lifetime_points" />
          </template>
        </el-table-column>
      </el-table>
      <el-button class="mt" type="primary" @click="saveTiers">保存等级阈值</el-button>
    </el-card>

    <el-card class="mt" shadow="never" header="积分获取规则">
      <el-form label-width="200px" class="form">
        <el-form-item label="每消耗 X 元得 1 分">
          <el-input v-model.number="rulesForm.spend_yuan_per_point" />
          <small class="hint">例如填入 10 表示每 ¥10 换 1 分（向下取整）</small>
        </el-form-item>
        <el-form-item label="生日额外倍数">
          <el-input v-model.number="rulesForm.birthday_multiplier" />
        </el-form-item>
        <el-form-item label="活动倍数">
          <el-input v-model.number="rulesForm.campaign_multiplier" />
        </el-form-item>
        <el-form-item label="活动截止日期">
          <el-input v-model="rulesForm.campaign_end" placeholder="yyyy-mm-dd（留空=无额外活动倍数）" />
        </el-form-item>
        <el-form-item label="自然入账年的到期清零规则">
          <el-input v-model.number="rulesForm.expiry_jan_clear_after_years" />
          <small class="hint">
            每笔入账记录在「入账自然年首日 + (该项+1) 年」的 1 月 1 日到期；预置脚本与后端逻辑保持一致。
          </small>
        </el-form-item>
        <el-form-item label="活动名称文案">
          <el-input v-model="rulesForm.active_event_name" />
        </el-form-item>
      </el-form>
      <el-button type="primary" @click="saveRules">保存积分规则</el-button>
    </el-card>
  </div>
</template>

<style scoped>
.mt {
  margin-top: 12px;
}
.muted {
  color: var(--el-text-color-secondary);
  margin: 0;
}
.form {
  max-width: 760px;
}
.hint {
  display: block;
  color: var(--el-text-color-secondary);
}
</style>
