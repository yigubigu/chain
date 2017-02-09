import mockHsmCreateSaga from 'MockHsm/Form/mockHsmCreateSaga'

export default function* rootSaga() {
  yield [
    mockHsmCreateSaga(),
  ]
}
