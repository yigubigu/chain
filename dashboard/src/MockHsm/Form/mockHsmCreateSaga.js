import { call, put, takeEvery } from 'redux-saga/effects'
import { chainClient } from 'utility/environment'

function* createMockHsmKey(action) {
  console.log(action);
  const resp = yield chainClient().mockHsm.keys.create(action.body)
  yield put({type: 'SUCCESS_MOCK_HSM_KEY'})
}

export default function* () {
  yield takeEvery('CREATE_MOCK_HSM_KEY', createMockHsmKey)
}
