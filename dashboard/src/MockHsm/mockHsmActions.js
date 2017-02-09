import { baseListActions, baseCreateActions } from 'features/shared/actions'
import { chainClient } from 'utility/environment'

const type = 'mockhsm'

export default {
  ...baseCreateActions(type, {
    className: 'MockHsm',
    clientApi: () => chainClient().mockHsm.keys
  }),
  ...baseListActions(type, { className: 'MockHsm' }),

  createKey: body => ({ type: 'CREATE_MOCK_HSM_KEY', body })
}
