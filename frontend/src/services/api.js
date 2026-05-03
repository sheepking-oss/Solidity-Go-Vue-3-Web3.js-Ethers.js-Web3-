import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json'
  }
})

api.interceptors.response.use(
  (response) => response,
  (error) => {
    console.error('API Error:', error)
    return Promise.reject(error)
  }
)

export const productApi = {
  async getProductTrace(serialNumber) {
    const response = await api.get('/products/trace', {
      params: { serial_number: serialNumber }
    })
    return response.data
  },

  async getProductByHash(hash) {
    const response = await api.get(`/products/hash/${hash}`)
    return response.data
  },

  async getAllProducts() {
    const response = await api.get('/products')
    return response.data
  },

  async healthCheck() {
    const response = await api.get('/health')
    return response.data
  }
}

export default api
