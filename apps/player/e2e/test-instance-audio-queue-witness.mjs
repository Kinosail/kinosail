export const faultWitness = () => ({
  handlerEntered: false, upstreamStarted: false, upstreamReturned: false,
  upstreamStatus: null, upstreamFailed: false, invalidStatus: false,
  holdRegistered: false, holdReleased: false, fulfillStarted: false,
  fulfilled: false, fulfillFailed: false, responseObserved: false, responseStatus: null,
});
function statusFact(facts, name, value) {
  if (Number.isInteger(value) && value >= 100 && value <= 599) facts[name] = value;
  else facts.invalidStatus = true;
}
export async function fetchQueueResponse(route, facts) {
  facts.handlerEntered = true;
  facts.upstreamStarted = true;
  try {
    const response = await route.fetch();
    facts.upstreamReturned = true;
    statusFact(facts, 'upstreamStatus', response.status());
    return response;
  } catch (error) {
    facts.upstreamFailed = true;
    throw error;
  }
}
async function deliver(route, facts, options) {
  facts.fulfillStarted = true;
  try {
    await route.fulfill(options);
    facts.fulfilled = true;
  } catch (error) {
    facts.fulfillFailed = true;
    throw error;
  }
}
export async function holdQueueResponse(route, response, facts, register) {
  await new Promise(resolve => {
    facts.holdRegistered = true;
    register(() => { facts.holdReleased = true; resolve(); });
  });
  await deliver(route, facts, {response});
}
export async function fulfillFailure(route, facts, headers) {
  facts.handlerEntered = true;
  await deliver(route, facts, {status: 503, headers});
}
export function observeFaultResponse(facts, response) {
  facts.responseObserved = true;
  statusFact(facts, 'responseStatus', response.status());
}
