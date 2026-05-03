<template>
  <div class="timeline-section" v-if="showContent">
    <div class="card">
      <div class="card-header timeline-header">
        <div class="timeline-header-content">
          <h2>产品溯源时间轴</h2>
          <div class="product-info" v-if="serialNumber">
            <span class="product-label">序列号：</span>
            <span class="product-serial">{{ serialNumber }}</span>
          </div>
        </div>
        <div class="status-badge" v-if="timelineData.length > 0">
          <span class="status-dot" :class="latestStatusClass"></span>
          {{ latestStatus }}
        </div>
      </div>

      <div class="card-body">
        <div v-if="loading" class="loading-container">
          <div class="loading-spinner-large"></div>
          <p>正在查询产品溯源信息...</p>
        </div>

        <div v-else-if="error" class="error-container animate-fade-in">
          <div class="error-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10" />
              <line x1="15" y1="9" x2="9" y2="15" />
              <line x1="9" y1="9" x2="15" y2="15" />
            </svg>
          </div>
          <h3>查询失败</h3>
          <p>{{ error }}</p>
        </div>

        <div v-else-if="timelineData.length === 0 && !loading && !error" class="empty-container">
          <div class="empty-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
              <line x1="9" y1="9" x2="15" y2="9" />
              <line x1="9" y1="15" x2="15" y2="15" />
            </svg>
          </div>
          <h3>暂无数据</h3>
          <p>请输入产品序列号进行查询</p>
        </div>

        <div v-else class="timeline-container animate-fade-in">
          <div class="timeline">
            <TimelineItem
              v-for="(item, index) in timelineData"
              :key="item.current_hash"
              :item="item"
              :index="index"
              :is-last="index === timelineData.length - 1"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import TimelineItem from './TimelineItem.vue'

const props = defineProps({
  timelineData: {
    type: Array,
    default: () => []
  },
  serialNumber: {
    type: String,
    default: ''
  },
  loading: {
    type: Boolean,
    default: false
  },
  error: {
    type: String,
    default: ''
  }
})

const showContent = computed(() => {
  return props.loading || props.error || props.timelineData.length > 0
})

const latestStatus = computed(() => {
  if (props.timelineData.length === 0) return ''
  return props.timelineData[props.timelineData.length - 1].status_text
})

const latestStatusClass = computed(() => {
  const status = latestStatus.value
  switch (status) {
    case 'Manufactured':
      return 'status-manufactured'
    case 'Shipped':
      return 'status-shipped'
    case 'Delivered':
      return 'status-delivered'
    default:
      return ''
  }
})
</script>

<style scoped>
.timeline-section {
  margin-top: 2rem;
}

.timeline-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 1rem;
}

.timeline-header-content h2 {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 0.5rem;
}

.product-info {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.product-label {
  font-size: 0.875rem;
  color: var(--text-secondary);
}

.product-serial {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--primary-color);
  background: rgba(99, 102, 241, 0.1);
  padding: 0.25rem 0.75rem;
  border-radius: var(--radius-sm);
}

.status-badge {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 1rem;
  background: var(--bg-primary);
  border-radius: var(--radius-md);
  font-weight: 500;
  color: var(--text-primary);
}

.status-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.status-dot.status-manufactured {
  background: var(--primary-color);
}

.status-dot.status-shipped {
  background: var(--warning-color);
}

.status-dot.status-delivered {
  background: var(--success-color);
}

.loading-container {
  text-align: center;
  padding: 4rem 2rem;
}

.loading-spinner-large {
  width: 48px;
  height: 48px;
  margin: 0 auto 1rem;
  border: 3px solid var(--border-color);
  border-radius: 50%;
  border-top-color: var(--primary-color);
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.loading-container p {
  color: var(--text-secondary);
}

.error-container {
  text-align: center;
  padding: 3rem 2rem;
}

.error-icon {
  width: 64px;
  height: 64px;
  margin: 0 auto 1.5rem;
  background: rgba(239, 68, 68, 0.1);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--error-color);
  padding: 1rem;
}

.error-container h3 {
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 0.5rem;
}

.error-container p {
  color: var(--text-secondary);
}

.empty-container {
  text-align: center;
  padding: 3rem 2rem;
}

.empty-icon {
  width: 64px;
  height: 64px;
  margin: 0 auto 1.5rem;
  background: var(--bg-primary);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-secondary);
  padding: 1rem;
}

.empty-container h3 {
  font-size: 1.125rem;
  font-weight: 500;
  color: var(--text-secondary);
  margin-bottom: 0.5rem;
}

.empty-container p {
  color: var(--text-secondary);
  font-size: 0.875rem;
}

.timeline-container {
  padding: 1rem 0;
}

.timeline {
  position: relative;
  padding-left: 2rem;
}

.timeline::before {
  content: '';
  position: absolute;
  left: 0.75rem;
  top: 0;
  bottom: 0;
  width: 2px;
  background: linear-gradient(to bottom, var(--primary-color), var(--secondary-color));
  border-radius: 1px;
}

@media (max-width: 768px) {
  .timeline-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .timeline-header-content h2 {
    font-size: 1.1rem;
  }

  .timeline::before {
    left: 0.5rem;
  }
}
</style>
