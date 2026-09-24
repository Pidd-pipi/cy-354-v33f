import request from '../utils/request'
import type { PageResult, Product } from '../types'

export interface ProductQuery {
  page?: number
  page_size?: number
  category?: string
  campus?: string
  keyword?: string
  status?: string
}

export function listProducts(params: ProductQuery) {
  return request.get<never, { code: number; message: string; data: PageResult<Product> }>('/products', { params })
}

export function getProduct(id: number) {
  return request.get<never, { code: number; message: string; data: Product }>(`/products/${id}`)
}

export function createProduct(data: Partial<Product>) {
  return request.post<never, { code: number; message: string; data: Product }>('/products', data)
}

export interface UpdateProductPayload {
  title: string
  description: string
  price: number
  condition: string
  trade_location: string
  version: number
}

export function updateProduct(id: number, data: UpdateProductPayload) {
  return request.put<never, { code: number; message: string; data: Product }>(`/products/${id}`, data)
}

export function removeProduct(id: number) {
  return request.delete<never, { code: number; message: string; data: Product }>(`/products/${id}`)
}

export function listGraduation() {
  return request.get<never, { code: number; message: string; data: PageResult<Product> }>('/products/graduation')
}
