<template>
  <div 
    class="timeline-item animate-slide-in" 
    :style="{ animationDelay: `${index * 0.1}s` }"
  >
    <div class="timeline-dot" :class="statusClass">
      <svg v-if="item.status_text === 'Manufactured'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
      </svg>
      <svg v-else-if="item.status_text === 'Shipped'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <rect x="1" y="3" width="15" height="13" />
        <polygon points="16 8 20 8 23 11 23 16 16 16 16 8" />
        <circle cx="5.5" cy="18.5" r="2.5" />
        <circle cx="18.5" cy="18.5" r="2.5" />
      </svg>
      <svg v-else-if="item.status_text === 'Delivered'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <polyline points="20 6 9 17 4 12" />
      </svg>
      <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <circle cx="12" cy="12" r="10" />
        <path d="M12 16v-4" />
        <path d="M12 8h.01" />
      </svg>
    </div>

    <div class="timeline-content card p-6" :class="`status-${item.status_text?.toLowerCase()}`">
      <div class="timeline-header">
        <div class="timeline-status">
          <span class="status-badge" :class="statusClass">
            {{ statusLabel }}
          </span>
        </div>
        <div class="timeline-time">
          <svg class="time-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="10" />
            <polyline points="12,6 12,12 16,14" />
          </svg>
          <span>{{ formattedTime }}</span>
        </div>
      </div>

      <div class="timeline-details">
        <div class="detail-row">
          <span class="detail-label">操作人</span>
          <span class="detail-value operator-address">
            {{ item.operator }}
          </span>
        </div>

        <div class="detail-row" v-if="item.block_number">
          <span class="detail-label">区块高度</span>
          <span class="detail-value">{{ item.block_number }}</span>
        </div>

        <div class="detail-row hash-row">
          <div class="hash-label">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
              <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
            </svg>
            交易哈希
          </div>
          <div class="hash-value">
            <span>{{ shortTxHash }}</span>
          </div>
        </div>
      </div>

      <div class="timeline-footer" v-if="item.previous_hash && item.previous_hash !== '0x0000000000000000000000000000000000000000000000000000000000000000'">
        <span class="chain-label">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
            <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
          </svg>
          链接至前一状态
        </span>
        <span class="hash-prev">{{ shortPreviousHash }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  item: {
    type: Object,
    required: true
  },
  index: {
    type: Number,
    required: true
  },
  isLast: {
    type: Boolean,
    default: false
  }
})

const statusClass = computed(() => {
  const status = props.item.status_text
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

const statusLabel = computed(() => {
  const status = props.item.status_text
  switch (status) {
    case 'Manufactured':
      return '已制造'
    case 'Shipped':
      return '已发货'
    case 'Delivered':
      return '已签收'
    default:
      return status
  }
})

const formattedTime = computed(() => {
  if (!props.item.timestamp) return ''
  const date = new Date(props.item.timestamp * 1000)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
})

const shortTxHash = computed(() => {
  const hash = props.item.transaction_hash
  if (!hash) return ''
  if (hash.length <= 18) return hash
  return `${hash.slice(0, 10)}...${hash.slice(-8)}`
})

const shortPreviousHash = computed(() => {
  const hash = props.item.previous_hash
  if (!hash) return ''
  if (hash.length <= 18) return hash
  return `${hash.slice(0, 10)}...${hash.slice(-8)}`
})
</script>

<style scoped>
.timeline-item {
  position: relative;
  padding-bottom: 2.5rem;
}

.timeline-item:last-child {
  padding-bottom: 0;
}

.timeline-dot {
  position: absolute;
  left: -2.75rem;
  top: 1.5rem;
  width: 2.5rem;
  height: 2.5rem;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 3px solid white;
  box-shadow: var(--shadow-md);
  transition: all 0.3s ease;
  z-index: 10;
}

.timeline-dot.status-manufactured {
  background: linear-gradient(135deg, var(--primary-color) 0%, var(--primary-dark) 100%);
}

.timeline-dot.status-shipped {
  background: linear-gradient(135deg, var(--warning-color) 0%, #d97706 100%);
}

.timeline-dot.status-delivered {
  background: linear-gradient(135deg, var(--success-color) 0%, #059669 100%);
}

.timeline-dot svg {
  width: 1.25rem;
  height: 1.25rem;
  color: white;
}

.timeline-content {
  position: relative;
  background: var(--bg-secondary);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-md);
  border: 1px solid var(--border-color);
  transition: all 0.3s ease;
}

.timeline-content:hover {
  box-shadow: var(--shadow-lg);
  transform: translateX(4px);
}

.timeline-content::before {
  content: '';
  position: absolute;
  left: -12px;
  top: 1.875rem;
  width: 0;
  height: 0;
  border-top: 8px solid transparent;
  border-bottom: 8px solid transparent;
  border-right: 12px solid var(--border-color);
}

.timeline-content::after {
  content: '';
  position: absolute;
  left: -10px;
  top: 1.875rem;
  width: 0;
  height: 0;
  border-top: 8px solid transparent;
  border-bottom: 8px solid transparent;
  border-right: 12px solid var(--bg-secondary);
}

.timeline-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 1rem;
  margin-bottom: 1.25rem;
}

.status-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.375rem 1rem;
  border-radius: 9999px;
  font-weight: 600;
  font-size: 0.875rem;
}

.status-badge.status-manufactured {
  background: linear-gradient(135deg, rgba(99, 102, 241, 0.1) 0%, rgba(79, 70, 229, 0.1) 100%);
  color: var(--primary-color);
}

.status-badge.status-shipped {
  background: linear-gradient(135deg, rgba(245, 158, 11, 0.1) 0%, rgba(217, 119, 6, 0.1) 100%);
  color: var(--warning-color);
}

.status-badge.status-delivered {
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.1) 0%, rgba(5, 150, 105, 0.1) 100%);
  color: var(--success-color);
}

.timeline-time {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--text-secondary);
  font-size: 0.875rem;
}

.time-icon {
  width: 1rem;
  height: 1rem;
}

.timeline-details {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.detail-row {
  display: flex;
  align-items: flex-start;
  gap: 1rem;
  padding: 0.75rem;
  background: var(--bg-primary);
  border-radius: var(--radius-md);
}

.detail-label {
  min-width: 80px;
  font-weight: 500;
  color: var(--text-secondary);
  font-size: 0.875rem;
  flex-shrink: 0;
}

.detail-value {
  color: var(--text-primary);
  font-weight: 500;
  word-break: break-all;
}

.operator-address {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 0.875rem;
  color: var(--primary-color);
  background: rgba(99, 102, 241, 0.05);
  padding: 0.125rem 0.5rem;
  border-radius: var(--radius-sm);
}

.hash-row {
  flex-direction: column;
  gap: 0.5rem;
}

.hash-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: 500;
  color: var(--text-secondary);
  font-size: 0.875rem;
}

.hash-label svg {
  width: 1rem;
  height: 1rem;
}

.hash-value {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 0.875rem;
  color: var(--text-primary);
  background: white;
  padding: 0.5rem 0.75rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-color);
}

.timeline-footer {
  margin-top: 1.25rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.chain-label {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  color: var(--text-secondary);
  font-size: 0.875rem;
}

.chain-label svg {
  width: 1rem;
  height: 1rem;
}

.hash-prev {
  font-family: 'SF Mono', 'Fira Code', monospace;
  font-size: 0.75rem;
  color: var(--text-secondary);
  background: var(--bg-primary);
  padding: 0.25rem 0.5rem;
  border-radius: var(--radius-sm);
}

@media (max-width: 768px) {
  .timeline-item {
    padding-bottom: 2rem;
  }

  .timeline-dot {
    left: -2.25rem;
    top: 1rem;
    width: 2rem;
    height: 2rem;
  }

  .timeline-dot svg {
    width: 1rem;
    height: 1rem;
  }

  .timeline-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.5rem;
  }

  .detail-row {
    flex-direction: column;
    gap: 0.25rem;
  }

  .detail-label {
    min-width: auto;
  }
}
</style>
