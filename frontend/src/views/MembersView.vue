<script setup>
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { api } from '../api/client'

const loading = ref(false)
const table = ref([])
const total = ref(0)
const pager = reactive({ page: 1, page_size: 20 })
const keyword = ref('')

const dlg = ref(false)
const form = reactive({
  phone: '',
  name: '',
  birthday: '',
  registered_at: '',
})

const dlgEarn = ref(false)
const dlgRedeem = ref(false)
const current = ref(null)
const earnForm = reactive({
  source_type: 'consumption',
  money_amount: null,
  points: null,
  reason: '',
  operator: '',
})
const redeemForm = reactive({
  points: null,
  source_type: 'redemption',
  product_name: '',
  redeem_amount: null,
  reason: '',
  operator: '',
})

async function fetchMembers() {
  loading.value = true
  try {
    const params = {
      page: pager.page,
      page_size: pager.page_size,
      q: keyword.value.trim() || undefined,
    }
    const { data } = await api.get('/members', { params })
    table.value = data.items ?? []
    total.value = data.total ?? table.value.length
  } catch (e) {
    ElMessage.error(e?.response?.data?.error || String(e.message || e))
  } finally {
    loading.value = false
  }
}

function submitCreate() {
  api.post('/members', { ...form }).then(() => {
    ElMessage.success('已录入会员')
    dlg.value = false
    fetchMembers()
  }).catch((e) => ElMessage.error(e?.response?.data?.error || '创建失败'))
}

function pickMember(row) {
  current.value = row
}

function submitEarn() {
  if (!current.value) return ElMessage.warning('请先选择会员')
  const body = {
    member_id: current.value.id,
    source_type: earnForm.source_type,
    money_amount:
      earnForm.source_type === 'consumption' ? Number(earnForm.money_amount) : undefined,
    points:
      earnForm.source_type === 'consumption'
        ? undefined
        : earnForm.points
          ? Number(earnForm.points)
          : undefined,
    reason: earnForm.reason,
    operator: earnForm.operator,
  }
  api.post('/points/earn', body).then(() => {
    ElMessage.success('已入账')
    dlgEarn.value = false
    fetchMembers()
  }).catch((e) => ElMessage.error(e?.response?.data?.error || '入账失败'))
}

function submitRedeem() {
  if (!current.value) return ElMessage.warning('请先选择会员')
  const body = {
    member_id: current.value.id,
    points: Number(redeemForm.points || 0),
    source_type: redeemForm.source_type,
    redeem_amount: redeemForm.redeem_amount ? Number(redeemForm.redeem_amount) : undefined,
    product_name: redeemForm.product_name,
    reason: redeemForm.reason,
    operator: redeemForm.operator,
  }
  api.post('/points/redeem', body).then(() => {
    ElMessage.success('已发起扣减')
    dlgRedeem.value = false
    fetchMembers()
  }).catch((e) => ElMessage.error(e?.response?.data?.error || '扣减失败'))
}

onMounted(fetchMembers)
</script>

<template>
  <div>
    <el-row :gutter="12">
      <el-col :span="14">
        <el-input v-model="keyword" clearable placeholder="按手机号或姓名检索" />
      </el-col>
      <el-col :span="10">
        <el-space>
          <el-button type="primary" @click="() => {pager.page = 1; fetchMembers()}">
            查询
          </el-button>
          <el-button type="success" plain @click="dlg = true">新增会员</el-button>
          <el-button :disabled="!current" plain @click="dlgEarn = true">发放积分</el-button>
          <el-button :disabled="!current" plain type="danger" @click="dlgRedeem = true">
            积分兑换 / 抵扣
          </el-button>
        </el-space>
      </el-col>
    </el-row>
    <el-table
      v-loading="loading"
      :data="table"
      class="mt"
      row-key="id"
      highlight-current-row
      @current-change="pickMember"
    >
      <el-table-column prop="phone" label="手机号" />
      <el-table-column prop="name" label="姓名" />
      <el-table-column prop="tier_name" label="等级" />
      <el-table-column prop="lifetime_points" label="累计入账" />
      <el-table-column prop="points_balance" label="当前余额" />
      <el-table-column prop="registered_at" label="注册日期">
        <template #default="{ row }">
          <span>{{ (row.registered_at || '').slice?.(0, 10) || row.registered_at }}</span>
        </template>
      </el-table-column>
    </el-table>
    <el-pagination
      v-model:current-page="pager.page"
      v-model:page-size="pager.page_size"
      class="pager"
      :total="total"
      layout="total, prev, pager, next, sizes"
      :page-sizes="[10, 20, 50]"
      background
      @current-change="fetchMembers"
      @size-change="
        () => {
          pager.page = 1
          fetchMembers()
        }
      "
    />

    <el-dialog v-model="dlg" title="录入会员">
      <el-form label-width="96px">
        <el-form-item label="手机号" required>
          <el-input v-model="form.phone" />
        </el-form-item>
        <el-form-item label="姓名" required>
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="生日">
          <el-input v-model="form.birthday" placeholder="yyyy-mm-dd（可选）" />
        </el-form-item>
        <el-form-item label="注册日期">
          <el-input v-model="form.registered_at" placeholder="留空则为今天 yyyy-mm-dd" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlg = false">取消</el-button>
        <el-button type="primary" @click="submitCreate">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="dlgEarn" title="积分发放">
      <p v-if="current" style="margin-top: -8px;">
        正在为 <b>{{ current.name }}</b> （{{ current.phone }}）入账
      </p>
      <el-form label-width="120px">
        <el-form-item label="积分来源类型">
          <el-select v-model="earnForm.source_type" placeholder="请选择">
            <el-option label="门店消费入账" value="consumption" />
            <el-option label="活动赠送" value="promo" />
            <el-option label="签到奖励" value="checkin" />
            <el-option label="手动补发" value="manual" />
          </el-select>
        </el-form-item>
        <template v-if="earnForm.source_type === 'consumption'">
          <el-form-item label="消费金额(元)">
            <el-input v-model.number="earnForm.money_amount" placeholder="按比例折算" />
          </el-form-item>
        </template>
        <template v-else>
          <el-form-item label="基础积分">
            <el-input v-model.number="earnForm.points" placeholder="乘以生日/活动等规则" />
          </el-form-item>
        </template>
        <el-form-item label="说明">
          <el-input v-model="earnForm.reason" />
        </el-form-item>
        <el-form-item label="操作人">
          <el-input v-model="earnForm.operator" placeholder="如不填默认为 system" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlgEarn = false">取消</el-button>
        <el-button type="primary" @click="submitEarn">确认入账</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="dlgRedeem" title="积分兑换或抵扣扣减">
      <p v-if="current" style="margin-top: -8px;">
        {{ current.name }}（{{ current.phone }}）当前余额：<b>{{ current.points_balance }}</b>
      </p>
      <el-form label-width="120px">
        <el-form-item label="扣减方式">
          <el-select v-model="redeemForm.source_type">
            <el-option label="实物兑换(redemption)" value="redemption" />
            <el-option label="订单抵扣(deduct)" value="deduct" />
          </el-select>
        </el-form-item>
        <el-form-item label="扣减积分" required>
          <el-input v-model.number="redeemForm.points" placeholder="必填" />
        </el-form-item>
        <el-form-item label="礼品名称">
          <el-input v-model="redeemForm.product_name" />
        </el-form-item>
        <el-form-item label="对应金额">
          <el-input v-model.number="redeemForm.redeem_amount" placeholder="可选" />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="redeemForm.reason" />
        </el-form-item>
        <el-form-item label="操作人">
          <el-input v-model="redeemForm.operator" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dlgRedeem = false">取消</el-button>
        <el-button type="danger" plain @click="submitRedeem">确认扣减</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.mt {
  margin-top: 12px;
}
.pager {
  margin-top: 12px;
}
</style>
