import MockHsmList from './List/MockHsmList'
import MockHsmForm from './Form/MockHsmForm'
import { makeRoutes } from 'features/shared'

export default (store) => makeRoutes(
  store,
  'mockhsm',
  MockHsmList,
  MockHsmForm,
  null,
  {
    skipFilter: true,
    name: 'MockHSM keys'
  }
)
