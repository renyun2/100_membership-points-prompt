<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const title = computed(() => route.meta.title || '会员积分管理')
</script>

<template>
  <el-container class="layout">
    <el-aside width="220px">
      <div class="brand">会员积分</div>
      <el-menu :default-active="route.path" router>
        <el-menu-item index="/dashboard">统计分析</el-menu-item>
        <el-menu-item index="/members">会员档案</el-menu-item>
        <el-menu-item index="/rules">规则与等级</el-menu-item>
        <el-menu-item index="/balance">积分查询</el-menu-item>
      </el-menu>
    </el-aside>
    <el-container>
      <el-header class="header">
        <span class="header-title">{{ title }}</span>
      </el-header>
      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.layout {
  min-height: 100vh;
}
.brand {
  padding: 20px 16px;
  font-weight: 600;
  font-size: 15px;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.header {
  display: flex;
  align-items: center;
  border-bottom: 1px solid var(--el-border-color-lighter);
  background: var(--el-bg-color);
}
.header-title {
  font-size: 18px;
  font-weight: 500;
}
.main {
  background: var(--el-fill-color-blank);
}
</style>
