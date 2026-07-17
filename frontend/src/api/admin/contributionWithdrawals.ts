import { apiClient } from '../client'
import type { ContributionWithdrawal, ContributionWithdrawalStatus, PaginatedResponse } from '@/types'

export interface ContributionWithdrawalAdminFilters {
  status?: ContributionWithdrawalStatus | ''
  search?: string
}

export interface ReviewContributionWithdrawalRequest {
  status: 'paid' | 'rejected'
  review_note?: string
  payment_reference?: string
}

export async function list(
  page = 1,
  pageSize = 20,
  filters: ContributionWithdrawalAdminFilters = {},
): Promise<PaginatedResponse<ContributionWithdrawal>> {
  const { data } = await apiClient.get<PaginatedResponse<ContributionWithdrawal>>('/admin/contribution-withdrawals', {
    params: {
      page,
      page_size: pageSize,
      status: filters.status || undefined,
      search: filters.search || undefined,
    },
  })
  return data
}

export async function review(id: number, request: ReviewContributionWithdrawalRequest): Promise<ContributionWithdrawal> {
  const { data } = await apiClient.put<ContributionWithdrawal>(`/admin/contribution-withdrawals/${id}/status`, request)
  return data
}

export default { list, review }
