/// <reference types="vite/client" />

import 'axios'

declare module 'axios' {
  export interface AxiosRequestConfig {
    silent?: boolean
  }
}
