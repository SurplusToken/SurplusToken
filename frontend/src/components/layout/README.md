# Layout Components

Vue 3 layout components for the Sub2API frontend, built with Composition API, TypeScript, and TailwindCSS.

## Components

### 1. AppLayout.vue

Main application layout with a top horizontal navigation bar.

**Usage:**

```vue
<template>
  <AppLayout>
    <!-- Your page content here -->
    <h1>Dashboard</h1>
    <p>Welcome to your dashboard!</p>
  </AppLayout>
</template>

<script setup lang="ts">
import { AppLayout } from '@/components/layout'
</script>
```

**Features:**

- Top horizontal navigation (AppTopNav) for all roles
- Centered `max-w-7xl` content container for regular users; full-width pages for admins
- Onboarding tour wiring (admin_guide / user_guide)

---

### 2. AppTopNav.vue

Top horizontal navigation bar used for every role (there is no sidebar).

**Features:**

- Logo/brand on the left (site name/logo from appStore)
- Horizontal nav items with active-route highlighting:
  - Regular users: core items inline, the rest under a "More" dropdown
  - Admins on `/admin/*`: core admin items inline, the rest grouped into
    semantic dropdowns (channels / payments & orders / ops / security / system),
    plus a "User View" link back to `/dashboard`
  - Admins on user pages: user nav plus a highlighted "Admin Console" entry
- Right side: announcements bell, docs link, model plaza entry, locale switcher,
  subscription progress, theme toggle, balance pill, user dropdown
- Mobile (`<lg`): hamburger toggles a dropdown panel listing all items
  (admin items shown with group titles)
- Onboarding tour anchors (`sidebar-group-manage`, `sidebar-channel-manage`,
  `sidebar-my-keys`) are preserved for the interactive guides

**Used automatically by AppLayout** - no need to import separately.

---

### 3. Navigation item composables

Menu definitions and their filtering (feature flags, simple mode, custom pages)
live in composables so the top nav never duplicates logic:

- `useUserNavItems.ts` — user menu (`userNavItems`), also used for the admin's
  personal area; exports the shared `NavItem` type, `applyFeatureFlags`, and icons
- `useAdminNavItems.ts` — admin menu (`adminNavItems`), including collapsible
  groups (`children`/`expandOnly`) and feature-flag filtering

Shared building blocks: `TopNavDropdown.vue` (dropdown group), `BalancePill.vue`,
`UserMenu.vue`, `useHeaderBalance.ts`.

---

### 4. AuthLayout.vue

Simple centered layout for authentication pages (login/register).

**Usage:**

```vue
<template>
  <AuthLayout>
    <!-- Login/Register form content -->
    <h2 class="mb-6 text-2xl font-bold">Login</h2>

    <form @submit.prevent="handleLogin">
      <!-- Form fields -->
    </form>

    <!-- Optional footer slot -->
    <template #footer>
      <p>
        Don't have an account?
        <router-link to="/register" class="text-indigo-600 hover:underline"> Sign up </router-link>
      </p>
    </template>
  </AuthLayout>
</template>

<script setup lang="ts">
import { AuthLayout } from '@/components/layout'

function handleLogin() {
  // Login logic
}
</script>
```

**Features:**

- Centered card container
- Gradient background
- Logo/brand at top
- Main content slot
- Optional footer slot for links
- Fully responsive

---

## Route Configuration

To set page titles in the header, add meta to your routes:

```typescript
// router/index.ts
const routes = [
  {
    path: '/dashboard',
    component: DashboardView,
    meta: { title: 'Dashboard' }
  },
  {
    path: '/api-keys',
    component: ApiKeysView,
    meta: { title: 'API Keys' }
  }
  // ...
]
```

---

## Store Dependencies

These components use the following Pinia stores:

- **useAuthStore**: For user authentication state, role checking, and logout
- **useAppStore**: For sidebar state management and toast notifications

Make sure these stores are properly initialized in your app.

---

## Styling

All components use TailwindCSS utility classes. Make sure your `tailwind.config.js` includes the component paths:

```js
module.exports = {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}']
  // ...
}
```

---

## Icons

Components use HTML entity icons for simplicity:

- &#128200; Chart (Dashboard)
- &#128273; Key (API Keys)
- &#128202; Bar Chart (Usage)
- &#127873; Gift (Redeem)
- &#128100; User (Profile)
- &#128268; Admin
- &#128101; Users
- &#128193; Folder (Groups)
- &#127760; Globe (Accounts)
- &#128260; Network (Proxies)
- &#127991; Ticket (Redeem Codes)

You can replace these with your preferred icon library (e.g., Heroicons, Font Awesome) if needed.

---

## Mobile Responsiveness

All components are fully responsive:

- **AppTopNav**: Collapses the horizontal nav into a hamburger-triggered dropdown
  panel on small screens (`<lg`); balance pill hides on very small screens and its
  values move into the user dropdown
- **AuthLayout**: Adapts padding and card size for mobile devices

The top nav uses Tailwind's responsive breakpoints (`sm:`/`lg:`/`xl:`) to adjust behavior.
