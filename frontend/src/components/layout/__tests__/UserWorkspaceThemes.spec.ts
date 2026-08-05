import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const testDirectory = dirname(fileURLToPath(import.meta.url))
const layoutSource = readFileSync(resolve(testDirectory, '../UserWorkspaceLayout.vue'), 'utf8')
const switcherSource = readFileSync(resolve(testDirectory, '../UserThemeSwitcher.vue'), 'utf8')
const themeSource = readFileSync(resolve(testDirectory, '../../../styles/user-themes.css'), 'utf8')
const dashboardSource = readFileSync(resolve(testDirectory, '../../../views/user/DashboardView.vue'), 'utf8')
const paymentSource = readFileSync(resolve(testDirectory, '../../../views/user/PaymentView.vue'), 'utf8')
const subscriptionsSource = readFileSync(resolve(testDirectory, '../../../views/user/SubscriptionsView.vue'), 'utf8')
const ordersSource = readFileSync(resolve(testDirectory, '../../../views/user/UserOrdersView.vue'), 'utf8')
const redeemSource = readFileSync(resolve(testDirectory, '../../../views/user/RedeemView.vue'), 'utf8')

describe('UserWorkspaceLayout theme structures', () => {
  it('provides a dedicated business workspace rail and page context', () => {
    expect(layoutSource).toContain('class="user-workspace__business-rail"')
    expect(layoutSource).toContain('class="user-workspace__rail-link"')
    expect(layoutSource).toContain('class="user-workspace__rail-account"')
    expect(layoutSource).toContain('class="user-workspace__business-create"')
    expect(layoutSource).toContain('class="user-workspace__context"')
    expect(layoutSource).toContain('class="user-workspace__rail-subnav"')
    expect(layoutSource).toContain('class="user-workspace__rail-sublink"')
    expect(layoutSource).toContain("const billingNavItems = [")
    expect(layoutSource).toContain("{ path: '/purchase', label: '充值与订阅' }")
    expect(layoutSource).toContain("{ path: '/orders', label: '订单记录' }")
    expect(layoutSource).toContain("path: '/usage', activePaths: ['/usage'], label: '使用记录'")
    expect(layoutSource).toContain("if (['/purchase', '/subscriptions', '/orders', '/redeem'].includes(path))")
    expect(layoutSource).toMatch(
      /if \(\['\/purchase', '\/subscriptions', '\/orders', '\/redeem']\.includes\(path\)\) \{\s+if \(theme\.value === 'business'\) return null/
    )
    expect(layoutSource).toContain("if (theme.value === 'business') return null")
    expect(themeSource).toContain("padding-left: 16rem;")
    expect(themeSource).toContain(".user-workspace[data-user-theme='business'] .lingqu-keys__stats")
    expect(themeSource).toContain("grid-template-columns: repeat(4, minmax(0, 1fr)) !important;")
    expect(themeSource).toContain(".user-workspace[data-user-theme='business'] .user-workspace__rail-subnav")
    expect(themeSource).toMatch(
      /\.user-workspace\[data-user-theme='business'] \.user-workspace__business-rail \{[\s\S]*?overflow: visible;/
    )
    expect(themeSource).toMatch(
      /\.user-workspace\[data-user-theme='business'] \.user-workspace__rail-section \{[\s\S]*?overflow-y: auto;/
    )
    expect(themeSource).toMatch(
      /\.user-workspace\[data-user-theme='business'] \.user-workspace__summary \{[\s\S]*?margin-top: 1rem;[\s\S]*?border: 0;[\s\S]*?inset 0 0 0 1px #cfd8df,[\s\S]*?animation: none;/
    )
    expect(themeSource).toContain('font-family: inherit !important;')
    expect(paymentSource).toContain('<template>')
    expect(subscriptionsSource).toContain('<template>')
    expect(ordersSource).toContain('<template>')
    expect(redeemSource).toContain('<template>')
    expect(dashboardSource).toContain('<template>')
  })

  it('keeps only business and cartoon themes available to users', () => {
    expect(layoutSource).not.toContain('class="user-workspace__claude-masthead"')
    expect(layoutSource).toContain("theme === 'business' ? businessNavItems : navItems")
    expect(switcherSource).toContain("value: 'business'")
    expect(switcherSource).toContain("value: 'cartoon'")
    expect(switcherSource).not.toContain("value: 'claude'")
  })

  it('keeps the business rail hidden outside the business theme', () => {
    expect(themeSource).toContain('.user-workspace__business-rail,')
    expect(themeSource).toContain('display: none;')
  })
})
