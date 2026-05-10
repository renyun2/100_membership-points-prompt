import { createRouter, createWebHistory } from 'vue-router'
import DashboardView from '../views/DashboardView.vue'
import MembersView from '../views/MembersView.vue'
import RulesView from '../views/RulesView.vue'
import BalanceView from '../views/BalanceView.vue'

const routes = [
  { path: '/', redirect: '/dashboard' },
  { path: '/dashboard', component: DashboardView, meta: { title: '统计分析' } },
  { path: '/members', component: MembersView, meta: { title: '会员档案' } },
  { path: '/rules', component: RulesView, meta: { title: '规则与等级' } },
  { path: '/balance', component: BalanceView, meta: { title: '积分查询' } },
]

export default createRouter({ history: createWebHistory(), routes })
