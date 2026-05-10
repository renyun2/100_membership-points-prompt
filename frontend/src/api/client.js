import axios from 'axios'

export const api = axios.create({ baseURL: '/api', timeout: 90000 })

export function unwrap(res) {
  return res?.data ?? res
}
