export type AccountStatus = 'active' | 'incomplete' | 'deactivated'

export type Account = {
  id: string
  status: AccountStatus
  displayName: string
  phone: string
}

export type ProfileInput = {
  displayName: string
  phone: string
}

export type ProfileValidation
  = | { ok: true, value: ProfileInput }
    | { ok: false, errors: Array<'displayName' | 'phone'> }

export function isBlank(value: string | null | undefined): boolean {
  return value === null || value === undefined || value.trim() === ''
}

export function profileIsComplete(account: {
  displayName?: string | null
  phone?: string | null
}): boolean {
  return !isBlank(account.displayName) && !isBlank(account.phone)
}

export function normalizeStatus(status: string): AccountStatus {
  if (status === 'deactivated') {
    return 'deactivated'
  }
  if (status === 'active') {
    return 'active'
  }
  return 'incomplete'
}

export function deriveStatus(account: Account): AccountStatus {
  if (account.status === 'deactivated') {
    return 'deactivated'
  }
  if (!profileIsComplete(account)) {
    return 'incomplete'
  }
  return 'active'
}

export function canStartRental(status: AccountStatus): boolean {
  return status === 'active'
}

export function canPublishListing(_status: AccountStatus): boolean {
  return false
}

export function postAuthPath(input: {
  hasError: boolean
  authenticated: boolean
  profileComplete: boolean
}): '/auth/login' | '/auth/profile' | '/' {
  if (input.hasError || !input.authenticated) {
    return '/auth/login'
  }
  if (!input.profileComplete) {
    return '/auth/profile'
  }
  return '/'
}

export function validateProfile(input: ProfileInput): ProfileValidation {
  const displayName = input.displayName.trim()
  const phone = input.phone.trim()
  const errors: Array<'displayName' | 'phone'> = []

  if (isBlank(displayName)) {
    errors.push('displayName')
  }
  if (isBlank(phone)) {
    errors.push('phone')
  }
  if (errors.length > 0) {
    return { ok: false, errors }
  }
  return { ok: true, value: { displayName, phone } }
}
