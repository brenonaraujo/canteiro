import { describe, expect, it } from 'vitest'
import { createAccountClient } from '../../../app/composables/auth/client'
import { ACCESS_TOKEN_KEY } from '../../../app/composables/auth/session'

type AccountRow = {
  id: string
  status: string
  display_name: string
  phone: string
}

function memoryStorage(initial?: Record<string, string>) {
  const memory = new Map(Object.entries(initial ?? {}))
  return {
    getItem: (key: string) => memory.get(key) ?? null,
    setItem: (key: string, value: string) => {
      memory.set(key, value)
    },
    removeItem: (key: string) => {
      memory.delete(key)
    }
  }
}

function testIdentityBackend() {
  let session = false
  const account: AccountRow = {
    id: 'test-identity-1',
    status: 'incomplete',
    display_name: '',
    phone: ''
  }

  return async (input: string, init?: RequestInit): Promise<Response> => {
    const path = String(input)
    const method = (init?.method ?? 'GET').toUpperCase()
    if (path.endsWith('/account') && method === 'GET') {
      if (!session) {
        return new Response(null, { status: 401 })
      }
      return Response.json(account)
    }
    if (path.endsWith('/account') && method === 'PATCH') {
      if (!session) {
        return new Response(null, { status: 401 })
      }
      const body = JSON.parse(String(init?.body)) as {
        display_name: string
        phone: string
      }
      account.display_name = body.display_name
      account.phone = body.phone
      account.status = 'active'
      return Response.json(account)
    }
    if (path.endsWith('/auth/logout') && method === 'POST') {
      if (!session) {
        return new Response(null, { status: 401 })
      }
      session = false
      return new Response(null, { status: 204 })
    }
    if (path.endsWith('/account/deactivate') && method === 'POST') {
      if (!session) {
        return new Response(null, { status: 401 })
      }
      const body = JSON.parse(String(init?.body)) as { confirm?: boolean }
      if (body.confirm !== true) {
        return new Response(null, { status: 400 })
      }
      account.status = 'deactivated'
      session = false
      return new Response(null, { status: 204 })
    }
    if (path.endsWith('/auth/google') && method === 'GET') {
      session = true
      return new Response(null, { status: 204 })
    }
    return new Response(null, { status: 404 })
  }
}

describe('critical account flow (test identity)', () => {
  it('entrar → perfil → sair → desativar without a real Google account', async () => {
    const storage = memoryStorage()
    const fetch = testIdentityBackend()
    const client = createAccountClient({
      apiBase: 'http://localhost:8080',
      storage,
      fetch
    })

    await expect(client.fetchMe()).rejects.toMatchObject({ status: 401 })

    await fetch('http://localhost:8080/auth/google')
    storage.setItem(ACCESS_TOKEN_KEY, 'test-identity')

    const created = await client.fetchMe()
    expect(created.status).toBe('incomplete')
    expect(created.displayName).toBe('')

    const saved = await client.saveProfile({
      displayName: 'QA Teste',
      phone: '11988887777'
    })
    expect(saved.status).toBe('active')
    expect(saved.displayName).toBe('QA Teste')

    await client.logout()
    expect(storage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
    await expect(client.fetchMe()).rejects.toMatchObject({ status: 401 })

    await fetch('http://localhost:8080/auth/google')
    storage.setItem(ACCESS_TOKEN_KEY, 'test-identity')
    await client.deactivate()
    expect(storage.getItem(ACCESS_TOKEN_KEY)).toBeNull()
    await expect(client.fetchMe()).rejects.toMatchObject({ status: 401 })
  })
})
