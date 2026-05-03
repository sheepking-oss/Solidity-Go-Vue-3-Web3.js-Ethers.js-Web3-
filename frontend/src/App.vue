<template>
  <div class="app">
    <header class="header">
      <div class="container">
        <div class="header-content">
          <div class="logo">
            <div class="logo-icon">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
                <polyline points="9 12 11 14 15 10" />
              </svg>
            </div>
            <div class="logo-text">
              <h1>供应链溯源系统</h1>
              <span>Supply Chain Traceability</span>
            </div>
          </div>
        </div>
      </div>
    </header>

    <main class="main">
      <div class="container">
        <ProductSearch @trace-start="handleTraceStart" />
        
        <TimelineView 
          :timeline-data="timelineData"
          :serial-number="currentSerialNumber"
          :loading="isLoading"
          :error="errorMessage"
        />
      </div>
    </main>

    <footer class="footer">
      <div class="container">
        <p>基于区块链技术的去中心化供应链溯源系统</p>
        <p class="footer-note">数据不可篡改 · 透明可追溯 · 安全可信</p>
      </div>
    </footer>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import ProductSearch from './components/ProductSearch.vue'
import TimelineView from './components/TimelineView.vue'
import { productApi } from './services/api'

const timelineData = ref([])
const currentSerialNumber = ref('')
const isLoading = ref(false)
const errorMessage = ref('')

const handleTraceStart = async (serialNumber) => {
  isLoading.value = true
  errorMessage.value = ''
  timelineData.value = []
  currentSerialNumber.value = serialNumber

  try {
    const response = await productApi.getProductTrace(serialNumber)
    timelineData.value = response.timeline || []
    
    if (timelineData.value.length === 0) {
      errorMessage.value = '未找到该产品的溯源信息，请确认序列号是否正确'
    }
  } catch (error) {
    if (error.response?.status === 404) {
      errorMessage.value = '未找到该产品的溯源信息，请确认序列号是否正确'
    } else if (error.response?.status === 400) {
      errorMessage.value = error.response.data?.message || '请求参数错误'
    } else {
      errorMessage.value = '网络错误，请稍后重试'
    }
    console.error('Trace error:', error)
  } finally {
    isLoading.value = false
  }
}
</script>

<style scoped>
.app {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

.header {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
  position: sticky;
  top: 0;
  z-index: 100;
}

.header-content {
  padding: 1rem 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.logo {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.logo-icon {
  width: 48px;
  height: 48px;
  background: linear-gradient(135deg, var(--primary-color) 0%, var(--primary-dark) 100%);
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  padding: 0.75rem;
}

.logo-text h1 {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 0.125rem;
}

.logo-text span {
  font-size: 0.75rem;
  color: var(--text-secondary);
}

.main {
  flex: 1;
  padding: 3rem 0;
}

.footer {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  padding: 1.5rem 0;
  text-align: center;
  color: var(--text-secondary);
  font-size: 0.875rem;
}

.footer-note {
  margin-top: 0.5rem;
  font-size: 0.75rem;
}

@media (max-width: 768px) {
  .logo-icon {
    width: 40px;
    height: 40px;
    padding: 0.6rem;
  }

  .logo-text h1 {
    font-size: 1.1rem;
  }

  .main {
    padding: 2rem 0;
  }
}
</style>
