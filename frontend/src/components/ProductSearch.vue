<template>
  <div class="search-section animate-fade-in">
    <div class="card p-8">
      <div class="search-header text-center mb-8">
        <div class="search-icon">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="11" cy="11" r="8" />
            <path d="m21 21-4.35-4.35" />
          </svg>
        </div>
        <h2>查询产品溯源信息</h2>
        <p>输入产品序列号，查看完整的供应链溯源历史</p>
      </div>

      <form @submit.prevent="handleSearch" class="search-form">
        <div class="form-group">
          <label for="serialNumber" class="form-label">产品序列号</label>
          <input
            id="serialNumber"
            v-model="serialNumber"
            type="text"
            class="form-input"
            placeholder="请输入产品序列号，如：PROD-2024-001"
            :disabled="isLoading"
            @input="clearError"
          />
          <p v-if="inputError" class="error-text">{{ inputError }}</p>
        </div>

        <button
          type="submit"
          class="btn btn-primary w-full"
          :disabled="isLoading || !serialNumber.trim()"
        >
          <span v-if="isLoading" class="loading-spinner"></span>
          <span v-else>
            <svg class="btn-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="11" cy="11" r="8" />
              <path d="m21 21-4.35-4.35" />
            </svg>
            查询溯源信息
          </span>
        </button>
      </form>

      <div class="search-tips mt-6">
        <h3>使用提示</h3>
        <ul>
          <li>
            <svg class="tip-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10" />
              <path d="M12 16v-4" />
              <path d="M12 8h.01" />
            </svg>
            产品序列号是产品的唯一标识
          </li>
          <li>
            <svg class="tip-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10" />
              <path d="M12 16v-4" />
              <path d="M12 8h.01" />
            </svg>
            区块链数据不可篡改，确保溯源信息真实可信
          </li>
          <li>
            <svg class="tip-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10" />
              <path d="M12 16v-4" />
              <path d="M12 8h.01" />
            </svg>
            时间轴展示产品从制造到交付的完整生命周期
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, defineEmits } from 'vue'

const emit = defineEmits(['trace-start'])

const serialNumber = ref('')
const isLoading = ref(false)
const inputError = ref('')

const clearError = () => {
  inputError.value = ''
}

const handleSearch = () => {
  if (!serialNumber.value.trim()) {
    inputError.value = '请输入产品序列号'
    return
  }

  isLoading.value = true
  inputError.value = ''

  emit('trace-start', serialNumber.value.trim())

  setTimeout(() => {
    isLoading.value = false
  }, 500)
}
</script>

<style scoped>
.search-section {
  margin-bottom: 3rem;
}

.search-header {
  max-width: 500px;
  margin: 0 auto;
}

.search-icon {
  width: 64px;
  height: 64px;
  margin: 0 auto 1.5rem;
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.1) 0%, rgba(79, 70, 229, 0.1) 100%);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--primary-color);
  padding: 1rem;
}

.search-header h2 {
  font-size: 1.5rem;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 0.5rem;
}

.search-header p {
  color: var(--text-secondary);
  font-size: 0.95rem;
}

.search-form {
  max-width: 500px;
  margin: 0 auto;
}

.form-group {
  margin-bottom: 1.5rem;
}

.form-label {
  display: block;
  font-weight: 500;
  color: var(--text-primary);
  margin-bottom: 0.5rem;
}

.error-text {
  color: var(--error-color);
  font-size: 0.875rem;
  margin-top: 0.5rem;
}

.btn-icon {
  width: 20px;
  height: 20px;
  margin-right: 0.5rem;
}

.loading-spinner {
  display: inline-block;
  width: 20px;
  height: 20px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-radius: 50%;
  border-top-color: white;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.search-tips {
  max-width: 500px;
  margin: 2rem auto 0;
  padding: 1.5rem;
  background: var(--bg-primary);
  border-radius: var(--radius-md);
}

.search-tips h3 {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 1rem;
}

.search-tips ul {
  list-style: none;
}

.search-tips li {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  color: var(--text-secondary);
  font-size: 0.875rem;
  margin-bottom: 0.75rem;
}

.search-tips li:last-child {
  margin-bottom: 0;
}

.tip-icon {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
  margin-top: 2px;
  color: var(--primary-color);
}

@media (max-width: 768px) {
  .search-header h2 {
    font-size: 1.25rem;
  }

  .search-icon {
    width: 56px;
    height: 56px;
    padding: 0.875rem;
  }
}
</style>
